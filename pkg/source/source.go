package source

// CacheSource represents the source of the cache data
type CacheSource string

const (
	Computed CacheSource = "computed and stored in cache"
	Hit      CacheSource = "cache"
)
