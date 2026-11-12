package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type testCase struct {
	File       string `json:"file"`
	SourceGoal string `json:"sourceGoal"`
	Phase      string `json:"phase,omitempty"`
	Error      string `json:"error,omitempty"`
}

func (test testCase) label() string {
	return fmt.Sprintf("%s (%s)", test.File, test.SourceGoal)
}

type testConfig struct {
	TimeoutMS   int                 `json:"timeoutMs"`
	SourceGoals map[string][]string `json:"sourceGoals"`
	Negative    []testCase          `json:"negative"`
}

func validSourceGoal(goal string) bool {
	return goal == "script" || goal == "module"
}

func loadCases(root string) ([]testCase, time.Duration, error) {
	file, err := os.Open(filepath.Join(root, "cases.json"))
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	config := testConfig{TimeoutMS: 5000}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, 0, err
	}
	if config.TimeoutMS <= 0 {
		return nil, 0, fmt.Errorf("timeoutMs must be positive")
	}

	scripts, err := os.ReadDir(filepath.Join(root, DIR_NAME_TEST_SCRIPTS))
	if err != nil {
		return nil, 0, err
	}
	cases := []testCase{}
	discovered := map[string]bool{}
	for _, script := range scripts {
		if script.IsDir() || filepath.Ext(script.Name()) != ".js" {
			continue
		}
		name := script.Name()
		discovered[name] = true
		goals, configured := config.SourceGoals[name]
		if !configured {
			goals = []string{"script"}
		}
		if len(goals) == 0 {
			return nil, 0, fmt.Errorf("no source goals configured for %s", name)
		}
		seenGoals := map[string]bool{}
		for _, goal := range goals {
			if !validSourceGoal(goal) || seenGoals[goal] {
				return nil, 0, fmt.Errorf("invalid or duplicate source goal %q for %s", goal, name)
			}
			seenGoals[goal] = true
			cases = append(cases, testCase{File: filepath.Join(DIR_NAME_TEST_SCRIPTS, name), SourceGoal: goal})
		}
	}
	for name := range config.SourceGoals {
		if !discovered[name] {
			return nil, 0, fmt.Errorf("source goals configured for missing test %s", name)
		}
	}

	seenNegative := map[string]bool{}
	for _, test := range config.Negative {
		if !validSourceGoal(test.SourceGoal) || test.Error == "" {
			return nil, 0, fmt.Errorf("invalid negative-test metadata for %s", test.File)
		}
		if test.Phase != "parse" && test.Phase != "link" && test.Phase != "runtime" {
			return nil, 0, fmt.Errorf("invalid failure phase %q for %s", test.Phase, test.File)
		}
		if filepath.Dir(test.File) != "negative-scripts" || filepath.Ext(test.File) != ".js" || seenNegative[test.File] {
			return nil, 0, fmt.Errorf("invalid or duplicate negative-test entry %s", test.File)
		}
		info, err := os.Stat(filepath.Join(root, test.File))
		if err != nil {
			return nil, 0, err
		}
		if !info.Mode().IsRegular() {
			return nil, 0, fmt.Errorf("negative test is not a regular file: %s", test.File)
		}
		seenNegative[test.File] = true
		cases = append(cases, test)
	}
	if len(cases) == 0 {
		return nil, 0, fmt.Errorf("no JavaScript test cases found")
	}
	return cases, time.Duration(config.TimeoutMS) * time.Millisecond, nil
}
