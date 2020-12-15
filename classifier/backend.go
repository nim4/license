package classifier

import (
	"context"
	"fmt"
	"github.com/google/licenseclassifier"
	"github.com/google/licenseclassifier/commentparser"
	"github.com/google/licenseclassifier/commentparser/language"
	"github.com/google/licenseclassifier/tools/identify_license/results"
	"io/ioutil"
	"sync"
)

// BackendInterface is the interface each backend must implement.
type BackendInterface interface {
	ClassifyLicenses(filenames []string, headers bool) []error
	ClassifyLicensesWithContext(ctx context.Context, filenames []string, headers bool) []error
	GetResults() results.LicenseTypes
}

// Backend is an object that handles classifying a license.
type Backend struct {
	results    results.LicenseTypes
	mu         sync.Mutex
	classifier *licenseclassifier.License
}

// New creates a new backend working on the local filesystem.
func NewBackend() (*Backend, error) {
	lc, err := licenseclassifier.New(licenseclassifier.DefaultConfidenceThreshold)
	if err != nil {
		return nil, err
	}
	return &Backend{classifier: lc}, nil
}

// ClassifyLicenses runs the license classifier over the given file.
func (b *Backend) ClassifyLicenses(filenames []string) (errors []error) {
	// Create a pool from which tasks can later be started. We use a pool because the OS limits
	// the number of files that can be open at any one time.
	const numTasks = 1000
	task := make(chan bool, numTasks)
	for i := 0; i < numTasks; i++ {
		task <- true
	}

	errs := make(chan error, len(filenames))

	var wg sync.WaitGroup
	analyze := func(filename string) {
		defer func() {
			task <- true
			wg.Done()
		}()
		if err := b.classifyLicense(filename); err != nil {
			errs <- err
		}
	}

	for _, filename := range filenames {
		wg.Add(1)
		<-task
		go analyze(filename)
	}
	go func() {
		wg.Wait()
		close(task)
		close(errs)
	}()

	for err := range errs {
		errors = append(errors, err)
	}
	return errors
}

// ClassifyLicensesWithContext runs the license classifier over the given file;
// ensure that it will respect the timeout in the provided context.
func (b *Backend) ClassifyLicensesWithContext(ctx context.Context, filenames []string) (errors []error) {
	done := make(chan bool)
	go func() {
		errors = b.ClassifyLicenses(filenames)
		done <- true
	}()
	select {
	case <-ctx.Done():
		err := ctx.Err()
		errors = append(errors, err)
		return errors
	case <-done:
		return errors
	}
}

// classifyLicense is called by a Go-function to perform the actual
// classification of a license.
func (b *Backend) classifyLicense(filename string) error {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to read %q: %v", filename, err)
	}

	matchLoop := func(contents string) {
		for _, m := range b.classifier.MultipleMatch(contents, false) {
			b.mu.Lock()
			b.results = append(b.results, &results.LicenseType{
				Filename:   filename,
				Name:       m.Name,
				Confidence: m.Confidence,
				Offset:     m.Offset,
				Extent:     m.Extent,
			})
			b.mu.Unlock()
		}
	}
	if lang := language.ClassifyLanguage(filename); lang == language.Unknown {
		matchLoop(string(contents))
	} else {
		comments := commentparser.Parse(contents, lang)
		for ch := range comments.ChunkIterator() {
			matchLoop(ch.String())
		}
	}
	return nil
}

// GetResults returns the results of the classifications.
func (b *Backend) GetResults() results.LicenseTypes {
	return b.results
}

