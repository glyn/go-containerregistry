// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package layout

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"io"
	"io/ioutil"
)

var _ v1.ImageIndex = (*layoutIndex)(nil)

type layoutIndex struct {
	path     LayoutPath
	rawIndex []byte
}

// ImageIndex is a convenience function which constructs a LayoutPath and returns its v1.ImageIndex.
func ImageIndex(path string) (v1.ImageIndex, error) {
	lp, err := Read(path)
	if err != nil {
		return nil, err
	}
	return lp.ImageIndex()
}

// ImageIndex returns a v1.ImageIndex for the LayoutPath.
func (l LayoutPath) ImageIndex() (v1.ImageIndex, error) {
	rawIndex, err := ioutil.ReadFile(l.path("index.json"))
	if err != nil {
		return nil, err
	}

	idx := &layoutIndex{
		path:     l,
		rawIndex: rawIndex,
	}

	return idx, nil
}

func (i *layoutIndex) MediaType() (types.MediaType, error) {
	return types.OCIImageIndex, nil
}

func (i *layoutIndex) Digest() (v1.Hash, error) {
	return partial.Digest(i)
}

func (i *layoutIndex) IndexManifest() (*v1.IndexManifest, error) {
	var index v1.IndexManifest
	err := json.Unmarshal(i.rawIndex, &index)
	return &index, err
}

func (i *layoutIndex) RawManifest() ([]byte, error) {
	return i.rawIndex, nil
}

func (i *layoutIndex) Image(h v1.Hash) (v1.Image, error) {
	// Look up the digest in our manifest first to return a better error.
	desc, err := i.findDescriptor(h)
	if err != nil {
		return nil, err
	}

	if !isExpectedMediaType(desc.MediaType, types.OCIManifestSchema1, types.DockerManifestSchema2) {
		return nil, fmt.Errorf("unexpected media type for %v: %s", h, desc.MediaType)
	}

	img := &layoutImage{
		path: i.path,
		desc: *desc,
	}
	return partial.CompressedToImage(img)
}

func (i *layoutIndex) ImageIndex(h v1.Hash) (v1.ImageIndex, error) {
	// Look up the digest in our manifest first to return a better error.
	desc, err := i.findDescriptor(h)
	if err != nil {
		return nil, err
	}

	if !isExpectedMediaType(desc.MediaType, types.OCIImageIndex, types.DockerManifestList) {
		return nil, fmt.Errorf("unexpected media type for %v: %s", h, desc.MediaType)
	}

	rawIndex, err := i.path.Bytes(h)
	if err != nil {
		return nil, err
	}

	return &layoutIndex{
		path:     i.path,
		rawIndex: rawIndex,
	}, nil
}

func (i *layoutIndex) Blob(h v1.Hash) (io.ReadCloser, error) {
	return i.path.Blob(h)
}

func (i *layoutIndex) findDescriptor(h v1.Hash) (*v1.Descriptor, error) {
	im, err := i.IndexManifest()
	if err != nil {
		return nil, err
	}

	for _, desc := range im.Manifests {
		if desc.Digest == h {
			return &desc, nil
		}
	}

	return nil, fmt.Errorf("could not find descriptor in index: %s", h)
}

// TODO: Pull this out into methods on types.MediaType? e.g. instead, have:
// * mt.IsIndex()
// * mt.IsImage()
func isExpectedMediaType(mt types.MediaType, expected ...types.MediaType) bool {
	for _, allowed := range expected {
		if mt == allowed {
			return true
		}
	}
	return false
}
