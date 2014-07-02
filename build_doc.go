package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	TempDocFile = "out.mobi"
)

func buildDoc(htmlPath string, destPath string) error {
	inputDir := filepath.Dir(htmlPath)
	c := exec.Command("docker", "run", "-v", inputDir+":/source", "jagregory/kindlegen", filepath.Base(htmlPath), "-o", TempDocFile)
	o, err := c.CombinedOutput()
	log.Printf("kindlegen output: %s", o)
	if err != nil {
		// kindlegen returns 1 for warnings and 2 for fatal errors.
		if status, ok := err.(*exec.ExitError); !ok || status.Sys().(syscall.WaitStatus).ExitStatus() != 1 {
			return fmt.Errorf("Failed to build doc: %v", err)
		}
	}
	srcPath := filepath.Join(inputDir, TempDocFile)
	err = os.Rename(srcPath, destPath)
	if err != nil {
		return fmt.Errorf("Unable to move %v to %v: %v", srcPath, destPath, err)
	}
	return nil
}
