package catalog

// LoadDir loads, validates, and decodes an agent catalog from a local CUE module
// directory, bypassing the registry entirely. Validation uses the same
// evaluation step as a fetched module, so schema rejects fail with
// ErrInvalidCatalog. No version is resolved and no network is used, so it is
// never stale and never raises ErrUnavailable.
func LoadDir(dir string) (*Catalog, error) {
	return loadCatalogModule(dir)
}
