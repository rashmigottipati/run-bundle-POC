package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	action "github.com/operator-framework/operator-registry/alpha/action"
	declarativeconfig "github.com/operator-framework/operator-registry/alpha/declcfg"

	log "github.com/sirupsen/logrus"
)

const (
	channelEntryName = "api-operator.v1.0.1"
)

// FBCContext represents an FBC context that has bundle, package and channel
// properties and metadata for creating an FBC.
type FBCContext struct {
	BundleImage       string
	Package           string
	DefaultChannel    string
	FBCName           string
	FBCPath           string
	FBCDirContext     string
	ChannelSchema     string
	ChannelName       string
	ChannelEntries    []declarativeconfig.ChannelEntry
	DescriptionReader io.Reader
}

//createMinimalFBC generates an FBC by creating bundle, package and channel blobs.
func (f *FBCContext) createMinimalFBC() (*declarativeconfig.DeclarativeConfig, error) {
	var (
		declcfg        *declarativeconfig.DeclarativeConfig
		declcfgpackage *declarativeconfig.Package
		err            error
	)

	render := action.Render{
		Refs: []string{f.BundleImage},
	}

	// generate bundles by rendering the bundle objects.
	declcfg, err = render.Run(context.TODO())
	if err != nil {
		log.Errorf("error in rendering the bundle image: %v", err)
		return nil, err
	}

	if len(declcfg.Bundles) < 0 {
		log.Errorf("error in rendering the correct number of bundles: %v", err)
		return nil, err
	}
	// validate length of bundles and add them to declcfg.Bundles.
	if len(declcfg.Bundles) == 1 {
		declcfg.Bundles = []declarativeconfig.Bundle{*&declcfg.Bundles[0]}
	} else {
		return nil, errors.New("error in expected length of bundles")
	}

	// init packages
	init := action.Init{
		Package:           f.Package,
		DefaultChannel:    f.DefaultChannel,
		DescriptionReader: f.DescriptionReader,
	}

	// generate packages
	declcfgpackage, err = init.Run()
	if err != nil {
		log.Errorf("error in generating packages for the FBC: %v", err)
		return nil, err
	}
	declcfg.Packages = []declarativeconfig.Package{*declcfgpackage}

	// generate channels
	channel := declarativeconfig.Channel{
		Schema:  f.ChannelSchema,
		Name:    f.ChannelName,
		Package: f.Package,
		Entries: f.ChannelEntries,
	}

	declcfg.Channels = []declarativeconfig.Channel{channel}

	return declcfg, nil
}

// validateFBC converts the generated declarative config to a model and validates it.
func validateFBC(declcfg *declarativeconfig.DeclarativeConfig) error {
	// convert declarative config to model
	FBCmodel, err := declarativeconfig.ConvertToModel(*declcfg)
	if err != nil {
		log.Errorf("error converting the declarative config to mode: %v", err)
		return err
	}

	if err = FBCmodel.Validate(); err != nil {
		log.Errorf("error validating the generated FBC: %v", err)
		return err
	}

	return nil
}

// writeDecConfigToFile writes the generated declarative config to a file.
func (f *FBCContext) writeDecConfigToFile(declcfg *declarativeconfig.DeclarativeConfig) error {
	var buf bytes.Buffer
	err := declarativeconfig.WriteJSON(*declcfg, &buf)
	if err != nil {
		log.Errorf("error writing to JSON encoder: %v", err)
		return err
	}
	if err := os.MkdirAll(f.FBCDirContext, 0755); err != nil {
		log.Errorf("error creating a directory for FBC: %v", err)
		return err
	}
	fbcFilePath := filepath.Join(f.FBCPath, f.FBCName)
	file, err := os.OpenFile(fbcFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("error opening FBC file: %v", err)
		return err
	}

	defer file.Close()

	if _, err := file.WriteString(string(buf.Bytes()) + "\n"); err != nil {
		log.Errorf("error writing to FBC file: %v", err)
		return err
	}

	return nil
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Error(err)
	}

	// Create the FBC context.
	f := &FBCContext{
		BundleImage:       "quay.io/rashmigottipati/api-operator:1.0.1",
		Package:           "api-operator",
		FBCDirContext:     "testdata",
		FBCPath:           filepath.Join(wd, "testdata"),
		FBCName:           "testFBC",
		DescriptionReader: bytes.NewBufferString("foo"),
		DefaultChannel:    "foo",
		ChannelSchema:     "olm.channel",
		ChannelName:       "foo",
	}

	// create entries for channel blob
	entries := []declarativeconfig.ChannelEntry{
		{
			Name: channelEntryName,
		},
	}
	f.ChannelEntries = entries

	// generate an FBC
	declcfg, err := f.createMinimalFBC()
	if err != nil {
		log.Errorf("error creating a minimal FBC: %v", err)
		return
	}

	// write declarative config to file
	if err = f.writeDecConfigToFile(declcfg); err != nil {
		log.Errorf("error writing declarative config to file: %v", err)
		return
	}

	// validate the generated declarative config
	if err = validateFBC(declcfg); err != nil {
		log.Errorf("error validating the generated FBC: %v", err)
		return
	}
}
