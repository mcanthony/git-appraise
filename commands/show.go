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

package commands

import (
	"errors"
	"flag"
	"fmt"
	"github.com/google/git-appraise/review"
)

var showFlagSet = flag.NewFlagSet("show", flag.ExitOnError)
var showJsonOutput = showFlagSet.Bool("json", false, "Format the output as JSON")

// showReview prints the current code review.
func showReview(args []string) error {
	showFlagSet.Parse(args)
	args = showFlagSet.Args()

	var r *review.Review
	var err error
	if len(args) > 1 {
		return errors.New("Only showing a single review is supported.")
	}

	if len(args) == 1 {
		r = review.Get(args[0])
	} else {
		r, err = review.GetCurrent()
	}

	if err != nil {
		return fmt.Errorf("Failed to load the review: %v\n", err)
	}
	if r == nil {
		return errors.New("There is no matching review.")
	}
	if *showJsonOutput {
		return r.PrintJson()
	}
	return r.PrintDetails()
}

// showCmd defines the "show" subcommand.
var showCmd = &Command{
	Usage: func(arg0 string) {
		fmt.Printf("Usage: %s show (<commit>)\n", arg0)
	},
	RunMethod: func(args []string) error {
		return showReview(args)
	},
}
