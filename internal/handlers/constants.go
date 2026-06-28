package handlers

type (
	HandlerPass struct {
		DomainName,
		ThemeColor,
		IndexURL string
	}
)

const (
	maxAuthorLen = 256
	ellipsisLen  = 3

	bskyEmbedImages    = "app.bsky.embed.images#view"
	galleryImages      = "app.bsky.embed.gallery#view"
	bskyEmbedExternal  = "app.bsky.embed.external#view"
	bskyEmbedVideo     = "app.bsky.embed.video#view"
	bskyEmbedQuote     = "app.bsky.embed.recordWithMedia#view"
	bskyEmbedText      = "app.bsky.embed.record#view"
	bskyEmbedTextQuote = "app.bsky.embed.record#viewRecord"
	bskyEmbedList      = "app.bsky.graph.defs#listView"
	bskyEmbedFeed      = "app.bsky.feed.defs#generatorView"
	bskyEmbedPack      = "app.bsky.graph.defs#starterPackViewBasic"
	unknownType        = "unknownType"

	modList    = "app.bsky.graph.defs#modlist"
	curateList = "app.bsky.graph.defs#curatelist"
)
