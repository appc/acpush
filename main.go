// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/appc/acpush/Godeps/_workspace/src/github.com/coreos/rkt/rkt/config"
	"github.com/appc/acpush/Godeps/_workspace/src/github.com/spf13/cobra"

	"github.com/appc/acpush/lib"
)

var (
	flagDebug           bool
	flagInsecure        bool
	flagSystemConfigDir string
	flagLocalConfigDir  string

	cmdACPush = &cobra.Command{
		Use:   "acpush [OPTIONS] IMAGE SIGNATURE URL",
		Short: "A utility for pushing ACI files to remote servers",
		Run:   runACPush,
	}
)

func init() {
	cmdACPush.Flags().BoolVar(&flagDebug, "debug", false, "Enables debug messages")
	cmdACPush.Flags().BoolVar(&flagInsecure, "insecure", false, "Permits unencrypted traffic")
	cmdACPush.Flags().StringVar(&flagSystemConfigDir, "system-conf", "/usr/lib/rkt", "Directory for system configuration")
	cmdACPush.Flags().StringVar(&flagLocalConfigDir, "local-conf", "/etc/rkt", "Directory for local configuration")
}

func main() {
	cmdACPush.Execute()
}

func runACPush(cmd *cobra.Command, args []string) {
	if len(args) != 3 {
		cmd.Usage()
		os.Exit(1)
	}

	conf, err := config.GetConfigFrom(flagSystemConfigDir, flagLocalConfigDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(2)
	}

	err = lib.Uploader{
		Acipath:  args[0],
		Ascpath:  args[1],
		Uri:      args[2],
		Insecure: flagInsecure,
		Debug:    flagDebug,
		SetHTTPHeaders: func(r *http.Request) {
			if r.URL == nil {
				return
			}
			headerer, ok := conf.AuthPerHost[r.URL.Host]
			if !ok {
				if flagDebug {
					fmt.Fprintf(os.Stderr, "No auth present in config for domain %s.\n", r.URL.Host)
				}
				return
			}
			header := headerer.Header()
			for k, v := range header {
				r.Header[k] = append(r.Header[k], v...)
			}
		},
	}.Upload()

	if err != nil {
		fmt.Fprintf(os.Stderr, "err: %v\n", err)
		os.Exit(1)
	}
	if flagDebug {
		fmt.Fprintln(os.Stderr, "Upload successful")
	}
}
