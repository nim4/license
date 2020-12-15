package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/nim4/license/classifier"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	vendorPath := flag.String(
		"path",
		"./vendor/",
		"Path of 'vendor' directory",
	)
	outputPath := flag.String(
		"output",
		"",
		"Write per dependency license to 'json' file",
	)
	licenseFiles := flag.String(
		"files",
		"LICENSE,LICENSE.TXT,LICENSE.MD,COPYING",
		"Comma separated list of license file names - case insensitive",
	)
	timeout := flag.Duration(
		"timeout",
		5*time.Minute,
		"Max execution time of the license check",
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	done := make(chan bool)
	var err error
	go func() {
		configPath := filepath.Dir(strings.TrimSuffix(*vendorPath, "/")) + "/.license"
		c := classifier.New(configPath, strings.Split(*licenseFiles, ","))
		err = c.Process(ctx, *vendorPath, *outputPath)
		done <- true
	}()

	select {
	case <-ctx.Done():
		fmt.Println("Timeout while processing the licenses!")
		os.Exit(1)
	case <-done:
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		fmt.Println("Dependencies license check passed! Good job!")
	}

}
