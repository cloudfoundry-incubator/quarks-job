package main

import (
	"os"

	utils "code.cloudfoundry.org/quarks-utils/pkg/cmd"

	cmd "code.cloudfoundry.org/quarks-job/cmd/internal"
)

const (
	index = `---
title: "Quarks Job"
linkTitle: "Quarks Job"
weight: 20
description: >
    Quarks-operator
---
	`
)

func main() {
	docDir := os.Args[1]
	if err := utils.GenCLIDocsyMarkDown(cmd.NewOperatorCommand(), docDir, index); err != nil {
		panic(err)
	}
}
