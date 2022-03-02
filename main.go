package main

import (
	"bytes"
	"context"
	"encoding/json"
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

// Bundle represents a bundle with all of the properties and metadata
type Bundle struct {
	BundleImage       string
	Package           string
	DefaultChannel    string
	FBCPath           string
	FBCDirContext     string
	ChannelSchema     string
	ChannelName       string
	ChannelEntries    []declarativeconfig.ChannelEntry
	DescriptionReader io.Reader
}

func (b *Bundle) createMinimalFBC() (*declarativeconfig.DeclarativeConfig, error) {
	var (
		declcfg        *declarativeconfig.DeclarativeConfig
		declcfgpackage *declarativeconfig.Package
		err            error
	)

	render := action.Render{
		Refs: []string{b.BundleImage},
	}

	// generate bundle blob
	declcfg, err = render.Run(context.TODO())
	if err != nil {
		log.Errorf("error in rendering the bundle image: %v", err)
		return nil, err
	}

	var buf bytes.Buffer
	err = declarativeconfig.WriteJSON(*declcfg, &buf)
	if err != nil {
		log.Errorf("error writing to JSON encoder: %v", err)
		return nil, err
	}

	// declcfg.Bundles[0] = []declarativeconfig.Bundle{*declcfgpackage}

	// write bundle blob to FBC
	if err := os.WriteFile("testFBC", buf.Bytes(), 0644); err != nil {
		log.Errorf("failed to write bundle blob to file: %v", err)
		return nil, err
	}

	// generate package blob
	init := action.Init{
		Package:           b.Package,
		DefaultChannel:    b.DefaultChannel,
		DescriptionReader: b.DescriptionReader,
	}

	declcfgpackage, err = init.Run()
	packageblob, err := json.Marshal(declcfgpackage)
	if err != nil {
		log.Errorf("error marshaling package blob: %v", err)
		return nil, err
	}

	declcfg.Packages = []declarativeconfig.Package{*declcfgpackage}

	// fbcPath := filepath.Join(b.FBCDirContext, "testFBC")
	file, err := os.OpenFile("testFBC", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("error opening FBC file: %v", err)
		return nil, err
	}
	defer file.Close()
	if _, err := file.WriteString(string(packageblob) + "\n"); err != nil {
		log.Errorf("error writing to FBC file: %s", err)
		return nil, err
	}

	// generate channel blob
	channel := declarativeconfig.Channel{
		Schema:  b.ChannelSchema,
		Name:    b.ChannelName,
		Package: b.Package,
		Entries: b.ChannelEntries,
	}

	channelblob, err := json.Marshal(channel)
	if err != nil {
		log.Errorf("error marshaling to JSON: %s", err)
		return nil, err
	}

	declcfg.Channels = []declarativeconfig.Channel{channel}

	file, err = os.OpenFile("testFBC", os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("error opening file: %v", err)
		return nil, err
	}

	if _, err := file.WriteString(string(channelblob) + "\n"); err != nil {
		log.Errorf("error writing to FBC file: %v", err)
		return nil, err
	}

	return declcfg, nil
}

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

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Error(err)
	}

	// Create the bundle context.
	b := &Bundle{
		BundleImage:       "quay.io/rashmigottipati/api-operator:1.0.1",
		Package:           "api-operator",
		FBCPath:           filepath.Join(wd, "testdata"),
		FBCDirContext:     "testdata",
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
	b.ChannelEntries = entries

	// generate a minimal FBC
	declcfg, err := b.createMinimalFBC()
	if err != nil {
		log.Errorf("error creating a minimal FBC: %v", err)
		return
	}

	// validate the generated declarative config
	err = validateFBC(declcfg)
}
