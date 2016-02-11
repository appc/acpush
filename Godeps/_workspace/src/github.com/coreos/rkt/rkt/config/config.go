// Copyright 2015 The rkt Authors
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

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/appc/acpush/Godeps/_workspace/src/github.com/coreos/rkt/common"
)

// Headerer is an interface for getting additional HTTP headers to use
// when downloading data (images, signatures).
type Headerer interface {
	Header() http.Header
}

type BasicCredentials struct {
	User     string
	Password string
}

// Config is a single place where configuration for rkt frontend needs
// resides.
type Config struct {
	AuthPerHost                  map[string]Headerer
	DockerCredentialsPerRegistry map[string]BasicCredentials
}

type configParser interface {
	parse(config *Config, raw []byte) error
}

var (
	// configSubDirs is a map saying what kinds of configuration
	// (values) are acceptable in a config subdirectory (key)
	configSubDirs  = make(map[string][]string)
	parsersForKind = make(map[string]map[string]configParser)
)

func addParser(kind, version string, parser configParser) {
	if len(kind) == 0 {
		panic("empty kind string when registering a config parser")
	}
	if len(version) == 0 {
		panic("empty version string when registering a config parser")
	}
	if parser == nil {
		panic("trying to register a nil parser")
	}
	if _, err := getParser(kind, version); err == nil {
		panic(fmt.Sprintf("A parser for kind %q and version %q already exist", kind, version))
	}
	if _, ok := parsersForKind[kind]; !ok {
		parsersForKind[kind] = make(map[string]configParser)
	}
	parsersForKind[kind][version] = parser
}

func registerSubDir(dir string, kinds []string) {
	if len(dir) == 0 {
		panic("trying to register empty config subdirectory")
	}
	if len(kinds) == 0 {
		panic("kinds array cannot be empty when registering config subdir")
	}
	allKinds := toArray(toSet(append(configSubDirs[dir], kinds...)))
	sort.Strings(allKinds)
	configSubDirs[dir] = allKinds
}

func toSet(a []string) map[string]struct{} {
	s := make(map[string]struct{})
	for _, v := range a {
		s[v] = struct{}{}
	}
	return s
}

func toArray(s map[string]struct{}) []string {
	a := make([]string, len(s))
	for k := range s {
		a = append(a, k)
	}
	return a
}

// GetConfig gets the Config instance with configuration taken from
// default system path (see common.DefaultSystemConfigDir) overridden
// with configuration from default local path (see
// common.DefaultLocalConfigDir).
func GetConfig() (*Config, error) {
	return GetConfigFrom(common.DefaultSystemConfigDir, common.DefaultLocalConfigDir)
}

// GetConfigFrom gets the Config instance with configuration taken
// from given system path overridden with configuration from given
// local path.
func GetConfigFrom(system, local string) (*Config, error) {
	cfg := newConfig()
	for _, cd := range []string{system, local} {
		subcfg, err := GetConfigFromDir(cd)
		if err != nil {
			return nil, err
		}
		mergeConfigs(cfg, subcfg)
	}
	return cfg, nil
}

// GetConfigFromDir gets the Config instance with configuration taken
// from given directory.
func GetConfigFromDir(dir string) (*Config, error) {
	subcfg := newConfig()
	if valid, err := validDir(dir); err != nil {
		return nil, err
	} else if !valid {
		return subcfg, nil
	}
	if err := readConfigDir(subcfg, dir); err != nil {
		return nil, err
	}
	return subcfg, nil
}

func newConfig() *Config {
	return &Config{
		AuthPerHost:                  make(map[string]Headerer),
		DockerCredentialsPerRegistry: make(map[string]BasicCredentials),
	}
}

func readConfigDir(config *Config, dir string) error {
	for csd, kinds := range configSubDirs {
		d := filepath.Join(dir, csd)
		if valid, err := validDir(d); err != nil {
			return err
		} else if !valid {
			continue
		}
		configWalker := getConfigWalker(config, kinds, d)
		if err := filepath.Walk(d, configWalker); err != nil {
			return err
		}
	}
	return nil
}

func validDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("expected %q to be a directory", path)
	}
	return true, nil
}

func getConfigWalker(config *Config, kinds []string, root string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		return readFile(config, info, path, kinds)
	}
}

func readFile(config *Config, info os.FileInfo, path string, kinds []string) error {
	if valid, err := validConfigFile(info); err != nil {
		return err
	} else if !valid {
		return nil
	}
	if err := parseConfigFile(config, path, kinds); err != nil {
		return err
	}
	return nil
}

func validConfigFile(info os.FileInfo) (bool, error) {
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return false, filepath.SkipDir
	case mode.IsRegular():
		return filepath.Ext(info.Name()) == ".json", nil
	case mode&os.ModeSymlink == os.ModeSymlink:
		// TODO: support symlinks?
		return false, nil
	default:
		return false, nil
	}
}

type configHeader struct {
	RktVersion string `json:"rktVersion"`
	RktKind    string `json:"rktKind"`
}

func parseConfigFile(config *Config, path string, kinds []string) error {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var header configHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return err
	}
	if len(header.RktKind) == 0 {
		return fmt.Errorf("no rktKind specified in %q", path)
	}
	if len(header.RktVersion) == 0 {
		return fmt.Errorf("no rktVersion specified in %q", path)
	}
	kindOk := false
	for _, kind := range kinds {
		if header.RktKind == kind {
			kindOk = true
			break
		}
	}
	if !kindOk {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		kindsStr := strings.Join(kinds, `", "`)
		return fmt.Errorf("the configuration directory %q expects to have configuration files of kinds %q, but %q has kind of %q", dir, kindsStr, base, header.RktKind)
	}
	parser, err := getParser(header.RktKind, header.RktVersion)
	if err != nil {
		return err
	}
	if err := parser.parse(config, raw); err != nil {
		return fmt.Errorf("failed to parse %q: %v", path, err)
	}
	return nil
}

func getParser(kind, version string) (configParser, error) {
	parsers, ok := parsersForKind[kind]
	if !ok {
		return nil, fmt.Errorf("no parser available for configuration of kind %q", kind)
	}
	parser, ok := parsers[version]
	if !ok {
		return nil, fmt.Errorf("no parser available for configuration of kind %q and version %q", kind, version)
	}
	return parser, nil
}

func mergeConfigs(config *Config, subconfig *Config) {
	for host, headerer := range subconfig.AuthPerHost {
		config.AuthPerHost[host] = headerer
	}
	for registry, creds := range subconfig.DockerCredentialsPerRegistry {
		config.DockerCredentialsPerRegistry[registry] = creds
	}
}
