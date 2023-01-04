/*
Copyright 2022 EscherCloud.

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

package main

import (
	"flag"
	"os"

	"github.com/eschercloudai/unikorn/pkg/constants"
	"github.com/eschercloudai/unikorn/pkg/managers/cluster"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	options := &zap.Options{}
	options.BindFlags(flag.CommandLine)

	flag.Parse()

	log.SetLogger(zap.New(zap.UseFlagOptions(options)))

	logger := log.Log.WithName("main")

	logger.Info("service starting", "application", constants.Application, "version", constants.Version, "revision", constants.Revision)

	if err := cluster.Run(); err != nil {
		logger.Error(err, "controller error")
		os.Exit(1)
	}
}
