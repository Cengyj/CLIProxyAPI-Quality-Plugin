# Dependency notices

`reference-THIRD_PARTY_NOTICES.txt` is preserved from the reference checkout. `modules/` contains unmodified top-level license, copying, copyright and notice files from the engine dependencies. `DEPENDENCY_MANIFEST.json` records their versions and SHA-256 hashes. Modules needed solely by dependencies' own tests or development tools are not included.

These third-party notices do not imply a license grant for the reference application source; see `../NOTICE.md`.

`go-LICENSE.txt` preserves the installed Go toolchain license. The fhttp module does not ship its referenced top-level license file; the manifest records that gap without inferring a license for fork-specific changes.
