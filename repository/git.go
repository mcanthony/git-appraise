/*
Copyright 2015 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package repository contains helper methods for working with the Git repo.
package repository

import (
	"crypto/sha1"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const branchRefPrefix = "refs/heads/"

// Note represents the contents of a git-note
type Note []byte

// Run the given git command and return its stdout, or an error if the command fails.
func runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	return strings.Trim(string(out), "\n"), err
}

// Run the given git command using the same stdin, stdout, and stderr as the review tool.
func runGitCommandInline(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Run the given git command using the same stdin, stdout, and stderr as the review tool.
func runGitCommandInlineOrDie(args ...string) {
	err := runGitCommandInline(args...)
	if err != nil {
		log.Print("git", args)
		log.Fatal(err)
	}
}

// Run the given git command and return its stdout.
func runGitCommandOrDie(args ...string) string {
	out, err := runGitCommand(args...)
	if err != nil {
		log.Print("git", args)
		log.Fatal(out)
	}
	return out
}

// IsGitRepo determines if the current working directory is inside of a git repository.
func IsGitRepo() bool {
	_, err := runGitCommand("rev-parse")
	if err == nil {
		return true
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false
	}
	log.Fatal(err)
	return false
}

// GetRepoStateHash returns a hash which embodies the entire current state of a repository.
func GetRepoStateHash() string {
	stateSummary := runGitCommandOrDie("show-ref")
	return fmt.Sprintf("%x", sha1.Sum([]byte(stateSummary)))
}

// GetUserEmail returns the email address that the user has used to configure git.
func GetUserEmail() string {
	return runGitCommandOrDie("config", "user.email")
}

// HasUncommittedChanges returns true if there are local, uncommitted changes.
func HasUncommittedChanges() bool {
	out := runGitCommandOrDie("status", "--porcelain")
	if len(out) > 0 {
		return true
	}
	return false
}

// VerifyGitRefOrDie verifies that the supplied ref points to a known commit.
func VerifyGitRefOrDie(ref string) {
	runGitCommandOrDie("show-ref", "--verify", ref)
}

// GetHeadRef returns the ref that is the current HEAD.
func GetHeadRef() string {
	return runGitCommandOrDie("symbolic-ref", "HEAD")
}

// GetCommitHash returns the hash of the commit pointed to by the given ref.
func GetCommitHash(ref string) string {
	return runGitCommandOrDie("show", "-s", "--format=%H", ref)
}

// GetCommitMessage returns the message stored in the commit pointed to by the given ref.
func GetCommitMessage(ref string) string {
	return runGitCommandOrDie("show", "-s", "--format=%B", ref)
}

// IsAncestor determins if the first argument points to a commit that is an ancestor of the second.
func IsAncestor(ancestor, descendant string) bool {
	_, err := runGitCommand("merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false
	}
	log.Fatal(err)
	return false
}

// SwitchToRef changes the currently-checked-out ref.
func SwitchToRef(ref string) {
	// If the ref starts with "refs/heads/", then we have to trim that prefix,
	// or else we will wind up in a detached HEAD state.
	if strings.HasPrefix(ref, branchRefPrefix) {
		ref = ref[len(branchRefPrefix):]
	}
	runGitCommandOrDie("checkout", ref)
}

// MergeRef merges the given ref into the current one.
//
// The ref argument is the ref to merge, and fastForward indicates that the
// current ref should only move forward, as opposed to creating a bubble merge.
func MergeRef(ref string, fastForward bool) {
	args := []string{"merge"}
	if fastForward {
		args = append(args, "--ff", "--ff-only")
	} else {
		args = append(args, "--no-ff")
	}
	args = append(args, ref)
	runGitCommandInlineOrDie(args...)
}

// RebaseRef rebases the given ref into the current one.
func RebaseRef(ref string) {
	runGitCommandInlineOrDie("rebase", "-i", ref)
}

// ListCommitsBetween returns the list of commits between the two given revisions.
//
// The "from" parameter is the starting point (exclusive), and the "to" parameter
// is the ending point (inclusive). If the commit pointed to by the "from" parameter
// is not an ancestor of the commit pointed to by the "to" parameter, then the
// merge base of the two is used as the starting point.
//
// The generated list is in chronological order (with the oldest commit first).
func ListCommitsBetween(from, to string) []string {
	out := runGitCommandOrDie("rev-list", "--reverse", "--ancestry-path", from+".."+to)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// GetNotes uses the "git" command-line tool to read the notes from the given ref for a given revision.
func GetNotes(notesRef, revision string) []Note {
	var notes []Note
	rawNotes, err := runGitCommand("notes", "--ref", notesRef, "show", revision)
	if err != nil {
		// We just assume that this means there are no notes
		return nil
	}
	for _, line := range strings.Split(rawNotes, "\n") {
		notes = append(notes, Note([]byte(line)))
	}
	return notes
}

// AppendNote appends a note to a revision under the given ref.
func AppendNote(notesRef, revision string, note Note) {
	runGitCommandOrDie("notes", "--ref", notesRef, "append", "-m", string(note), revision)
}

// ListNotedRevisions returns the collection of revisions that are annotated by notes in the given ref.
func ListNotedRevisions(notesRef string) []string {
	var revisions []string
	notesList := strings.Split(runGitCommandOrDie("notes", "--ref", notesRef, "list"), "\n")
	for _, notePair := range notesList {
		noteParts := strings.SplitN(notePair, " ", 2)
		if len(noteParts) == 2 {
			objHash := noteParts[1]
			objType, err := runGitCommand("cat-file", "-t", objHash)
			// If a note points to an object that we do not know about (yet), then err will not
			// be nil. We can safely just ignore those notes.
			if err == nil && objType == "commit" {
				revisions = append(revisions, objHash)
			}
		}
	}
	return revisions
}

// PushNotes pushes git notes to a remote repo.
func PushNotes(remote, notesRefPattern string) error {
	refspec := fmt.Sprintf("%s:%s", notesRefPattern, notesRefPattern)

	// The push is liable to fail if the user forgot to do a pull first, so
	// we treat errors as user errors rather than fatal errors.
	err := runGitCommandInline("push", remote, refspec)
	if err != nil {
		return fmt.Errorf("Failed to push to the remote '%s': %v", remote, err)
	}
	return nil
}

func getRemoteNotesRef(remote, localNotesRef string) string {
	relativeNotesRef := strings.TrimPrefix(localNotesRef, "refs/notes/")
	return "refs/notes/" + remote + "/" + relativeNotesRef
}

// PullNotes fetches the contents of the given notes ref from a remote repo,
// and then merges them with the corresponding local notes using the
// "cat_sort_uniq" strategy.
func PullNotes(remote, notesRefPattern string) {
	remoteNotesRefPattern := getRemoteNotesRef(remote, notesRefPattern)
	fetchRefSpec := fmt.Sprintf("+%s:%s", notesRefPattern, remoteNotesRefPattern)
	runGitCommandInlineOrDie("fetch", remote, fetchRefSpec)

	remoteRefs := runGitCommandOrDie("ls-remote", remote, notesRefPattern)
	for _, line := range strings.Split(remoteRefs, "\n") {
		lineParts := strings.Split(line, "\t")
		if len(lineParts) == 2 {
			ref := lineParts[1]
			remoteRef := getRemoteNotesRef(remote, ref)
			runGitCommandOrDie("notes", "--ref", ref, "merge", remoteRef, "-s", "cat_sort_uniq")
		}
	}
}
