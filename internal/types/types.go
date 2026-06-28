package types

type (
	UserProfile struct {
		Handle         string `json:"handle"`
		DisplayName    string `json:"displayName"`
		Avatar         string `json:"avatar"`
		Description    string `json:"description"`
		CreatedAt      string `json:"createdAt"`
		FollowersCount int64  `json:"followersCount"`
		FollowsCount   int64  `json:"followsCount"`
		PostsCount     int64  `json:"postsCount"`
		Associated     struct {
			Labeler bool `json:"labeler"`
		} `json:"associated"`
	}

	APIDID struct {
		DID string `json:"did"`
	}

	APIThread struct {
		Thread struct {
			// This is the main post
			Post APIPost `json:"post"`
			// Parent, if this is a reply to an already existing post
			// Also a pointer, so if there is no reply, this is nil
			Parent *struct {
				Post APIPost `json:"post"`
			} `json:"parent"`
		} `json:"thread"`
	}

	APIFeed struct {
		View struct {
			DisplayName string    `json:"displayName"`
			Description string    `json:"description"`
			Avatar      string    `json:"avatar"`
			IndexedAt   string    `json:"indexedAt"`
			Creator     APIAuthor `json:"creator"`
			LikeCount   int64     `json:"likeCount"`
		} `json:"view"`

		IsOnline bool `json:"isOnline"`
		IsValid  bool `json:"isValid"`
	}

	APIList struct {
		List struct {
			Name        string    `json:"name"`
			Purpose     string    `json:"purpose"`
			Avatar      string    `json:"avatar"`
			Description string    `json:"description"`
			IndexedAt   string    `json:"indexedAt"`
			Creator     APIAuthor `json:"creator"`
			ItemCount   int64     `json:"listItemCount"`
		} `json:"list"`
	}

	APIPack struct {
		StarterPack struct {
			Record struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				CreatedAt   string `json:"createdAt"`
			} `json:"record"`

			Creator APIAuthor `json:"creator"`
		} `json:"starterPack"`
	}

	APIImages []struct {
		FullSize    string         `json:"fullsize"`
		Alt         string         `json:"alt"`
		AspectRatio APIAspectRatio `json:"aspectRatio"`
	}

	APIAuthor struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Avatar      string `json:"avatar"`
	}

	APIExternal struct {
		URI         string `json:"uri"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Thumb       string `json:"thumb"`
	}

	APIPost struct {
		URI string `json:"uri"`

		Author APIAuthor `json:"author"`

		// Text of the post
		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`

			Facets []struct {
				Features []struct {
					Type string `json:"$type"`
					URI  string `json:"uri"`
					Tag  string `json:"tag"`
					DID  string `json:"did"`
				} `json:"features"`

				Index struct {
					ByteStart int64 `json:"byteStart"`
					ByteEnd   int64 `json:"byteEnd"`
				} `json:"index"`
			} `json:"facets"`
		} `json:"record"`

		// Embeds of stuff, if any.
		Embed struct {
			Type string `json:"$type"`

			// If this is a quote, and if there are embeds,
			// they'll be here
			Media MediaData `json:"media"`

			External APIExternal `json:"external"`

			// This is a text quote
			Record struct {
				Type string `json:"$type"`

				// This is for starter packs (it contains the quotee's id)
				URI string `json:"uri"`

				// This is a quote with media
				Record struct {
					Value struct {
						Text   string `json:"text"`
						Facets []struct {
							Features []struct {
								Type string `json:"$type"`
								URI  string `json:"uri"`
								Tag  string `json:"tag"`
								DID  string `json:"did"`
							} `json:"features"`

							Index struct {
								ByteStart int64 `json:"byteStart"`
								ByteEnd   int64 `json:"byteEnd"`
							} `json:"index"`
						} `json:"facets"`
					} `json:"value"`

					Author APIAuthor `json:"author"`

					URI string `json:"uri"`

					// This is for starter packs
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"record"`

				Value struct {
					Text string `json:"text"`

					Facets []struct {
						Features []struct {
							Type string `json:"$type"`
							URI  string `json:"uri"`
							Tag  string `json:"tag"`
							DID  string `json:"did"`
						} `json:"features"`

						Index struct {
							ByteStart int64 `json:"byteStart"`
							ByteEnd   int64 `json:"byteEnd"`
						} `json:"index"`
					} `json:"facets"`
				} `json:"value"`

				Author APIAuthor `json:"author"`

				Embeds []struct {
					MediaData
					Media MediaData `json:"media"`

					Record struct {
						Type string `json:"$type"`

						// This is for starter packs
						URI string `json:"uri"`

						// This is for starter packs
						Record struct {
							Description string `json:"description"`
							Name        string `json:"name"`
						} `json:"record"`

						// This is for feeds
						DisplayName string `json:"displayName"`

						// This is for lists
						Purpose string `json:"purpose"`

						// Found in lists, starter packs, feeds
						Name        string    `json:"name"`
						Avatar      string    `json:"avatar"`
						Description string    `json:"description"`
						Creator     APIAuthor `json:"creator"`
					} `json:"record"`
				} `json:"embeds"`

				// This is for feeds
				DisplayName string `json:"displayName"`

				// This is for lists
				Purpose string `json:"purpose"`

				// Found in lists, starter packs, feeds
				Name        string    `json:"name"`
				Avatar      string    `json:"avatar"`
				Description string    `json:"description"`
				Creator     APIAuthor `json:"creator"`
			} `json:"record"`

			Images APIImages `json:"images"`

			// Gallery (10+ images)
			// Why is it called "items"? Who knows.
			Items APIImages `json:"items"`

			CID         string         `json:"cid"`
			Thumbnail   string         `json:"thumbnail"`
			AspectRatio APIAspectRatio `json:"aspectRatio"`
		} `json:"embed"`

		ReplyCount  int64 `json:"replyCount"`
		RepostCount int64 `json:"repostCount"`
		LikeCount   int64 `json:"likeCount"`
		QuoteCount  int64 `json:"quoteCount"`
	}

	MediaData struct {
		Type string `json:"$type"`

		Images APIImages `json:"images"`

		Items APIImages `json:"items"`

		External APIExternal `json:"external"`

		CID         string         `json:"cid"`
		Thumbnail   string         `json:"thumbnail"`
		AspectRatio APIAspectRatio `json:"aspectRatio"`
	}

	APIAspectRatio struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	}

	OEmbed struct {
		Version      string `json:"version"`
		Type         string `json:"type"`
		ProviderName string `json:"provider_name"`
		ProviderURL  string `json:"provider_url"`
		AuthorName   string `json:"author_name"`
	}

	RichActivityEncoded struct {
		Type     string `json:"t"`
		Handle   string `json:"h"`
		PostID   string `json:"p"`
		PhotoCut string `json:"c"`
	}

	RichActivity struct {
		ID               string                  `json:"id"`
		URL              string                  `json:"url"`
		URI              string                  `json:"uri"`
		CreatedAt        string                  `json:"created_at"`
		Language         string                  `json:"language"` // "en"
		Content          string                  `json:"content"`
		SpoilerText      string                  `json:"spoiler_text"` // Title
		Visibility       string                  `json:"visibility"`   // "public"
		Application      RichActivityApplication `json:"application"`
		Account          RichActivityAccount     `json:"account"`
		MediaAttachments []RichActivityMedia     `json:"media_attachments"`
	}

	RichActivityApplication struct {
		Name    string `json:"name"`
		Website string `json:"website"`
	}

	RichActivityMedia struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		URL         string `json:"url"`
		Preview     string `json:"preview_url"`
		Description string `json:"description"`
	}

	RichActivityAccount struct {
		ID           string `json:"id"`
		DisplayName  string `json:"display_name"`
		UserName     string `json:"username"`
		Acct         string `json:"acct"`
		URL          string `json:"url"`
		URI          string `json:"uri"`
		Avatar       string `json:"avatar"`
		AvatarStatic string `json:"avatar_static"`
	}

	// https://atproto.com/specs/did#did-documents
	PLCDirectory struct {
		AKA     []string `json:"alsoKnownAs"`
		Service []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Endpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	}

	// To reduce redundancy in the template
	OwnData struct {
		Type string `json:"type"`

		Author APIAuthor `json:"author"`

		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`

			Facets []struct {
				Features []struct {
					Type string `json:"$type"`
					URI  string `json:"uri"`
					Tag  string `json:"tag"`
					DID  string `json:"did"`
				} `json:"features"`

				Index struct {
					ByteStart int64 `json:"byteStart"`
					ByteEnd   int64 `json:"byteEnd"`
				} `json:"index"`
			} `json:"facets"`
		} `json:"record"`

		Images APIImages `json:"images"`

		External APIExternal `json:"external"`

		PDS         string `json:"pds"`
		VideoCID    string `json:"videoCID"`
		VideoDID    string `json:"videoDID"`
		VideoHelper string `json:"videoURI"`

		Description string `json:"description"`
		StatsForTG  string `json:"statsForTG"`

		Thumbnail   string         `json:"thumbnail"`
		AspectRatio APIAspectRatio `json:"aspectRatio"`

		ReplyCount  int64 `json:"replyCount"`
		RepostCount int64 `json:"repostCount"`
		LikeCount   int64 `json:"likeCount"`
		QuoteCount  int64 `json:"quoteCount"`

		IsVideo bool `json:"isVideo"`
		IsGif   bool `json:"isGif"`

		OriginalPostID string `json:"originalPostID"`

		CommonEmbeds struct {
			Purpose     string    `json:"purpose"`
			Name        string    `json:"name"`
			Avatar      string    `json:"avatar"`
			Description string    `json:"description"`
			Creator     APIAuthor `json:"creator"`
		} `json:"commonEmbeds"`
	}

	SortedAPIResponse struct {
		OriginalData APIThread `json:"originalData"`
		ParsedData   OwnData   `json:"parsedData"`
	}
)
