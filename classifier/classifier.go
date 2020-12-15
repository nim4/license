package classifier

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Classifier struct {
	licenseConfigFile string
	fileNames []string
}

func New(configFile string, fileNames []string) *Classifier {
	fmt.Println(configFile)
	return &Classifier{
		licenseConfigFile: configFile,
		fileNames: fileNames,
	}
}

func classify(ctx context.Context, files []string) (map[string]string, error) {
	be, err := NewBackend()
	if err != nil {
		return nil, err
	}

	if errs := be.ClassifyLicensesWithContext(ctx, files); errs != nil {
		for _, err := range errs {
			return nil, err
		}
	}

	results := be.GetResults()
	if len(results) == 0 {
		return nil, errors.New("couldn't classify license(s)")
	}

	m := make(map[string]string, len(results))
	for _, r := range results {
		m[r.Filename] = r.Name
	}

	return m, nil
}


type set map[string]bool

type Dir struct {
	path  string
	dirs  set
	files set
}

func (d Dir) String() string {
	return fmt.Sprintf("Path: %s\nDir:\n%vFile:\n%v\n", d.path, d.dirs, d.files)
}

func mustDir(path string) Dir {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		log.Fatalf("Error reading %q directory: %v", path, err)
	}
	dir := Dir{
		path:  path,
		dirs:  make(set),
		files: make(set),
	}
	for _, fi := range fis {
		if fi.IsDir() {
			dir.dirs[fi.Name()] = true
			continue
		}

		dir.files[strings.ToUpper(fi.Name())] = true
	}
	return dir
}

func (c *Classifier) isLicensed(f set) string {
	for _, filename := range c.fileNames {
		if f[filename] {
			return filename
		}
	}
	return ""
}

func  (c *Classifier) hasLicense(dir Dir, licenceFiles *[]string) bool {
	if licenceFile := c.isLicensed(dir.files); licenceFile != "" {
		*licenceFiles = append(*licenceFiles, filepath.Join(dir.path, licenceFile))
		return true
	}

	for subDir := range dir.dirs {
		path := filepath.Join(dir.path, subDir)
		if !c.hasLicense(mustDir(path), licenceFiles) {
			return false
		}
	}

	return len(dir.dirs) > 0
}

func  (c *Classifier) allowedLicenses() set {
	allowed := make(set)
	f, err := os.Open(c.licenseConfigFile)
	if err != nil {
		return allowed
	}

	scanner := bufio.NewScanner(bufio.NewReader(f))
	for scanner.Scan() {
		license := scanner.Text()
		if len(license) == 0 || strings.HasPrefix(license, "#") {
			continue
		}
		allowed[license] = true
	}
	return allowed
}

func  (c *Classifier) Process(ctx context.Context, vendorPath string, outputPath string) error {
	output := make(map[string][]string)
	var licenseFiles []string
	hosts := mustDir(vendorPath)
	for host := range hosts.dirs {
		path := filepath.Join(hosts.path, host)
		repos := mustDir(path)
		for repo := range repos.dirs {
			repoPath := filepath.Join(repos.path, repo)

			if !c.hasLicense(mustDir(repoPath), &licenseFiles) {
				output["Unknown"] = append(output["Unknown"], strings.TrimPrefix(repoPath, vendorPath))
			}
		}
	}

	licences, err := classify(ctx, licenseFiles)
	if err != nil {
		return err
	}

	for path, licence := range licences {
		output[licence] = append(output[licence], strings.TrimPrefix(path, vendorPath))
	}

	if outputPath != "" {
		b, err := json.MarshalIndent(output, "", "    ")
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(outputPath, b, 0600)
		if err != nil {
			return err
		}
	}

	allowed := c.allowedLicenses()
	if len(allowed) > 0 {
		for licence, dependencies := range output {
			if !allowed[licence] {
				err = errors.New("forbidden license")
				log.Printf("This dependencies use forbidden license %q: %s\n", licence, strings.Join(dependencies, ", "))
			}
		}
	}

	return err
}
