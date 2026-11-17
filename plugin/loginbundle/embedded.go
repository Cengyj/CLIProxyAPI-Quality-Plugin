//go:build loginbundle_embedded

package loginbundle

import _ "embed"

//go:embed payload/runtime.tar.gz
var runtimeArchive []byte

func embeddedSpec() bundleSpec {
	spec := generatedSpec
	spec.archive = runtimeArchive
	return spec
}
