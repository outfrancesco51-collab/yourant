package anilist

import (
	"errors"
	"sort"
	"strings"
)

var (
	ErrAnimeNotFound = errors.New("anime not found in catalog")
)

// SeedCatalog contains 10 rich anime titles for offline and fallback operation.
var SeedCatalog = []AnimeMedia{
	{
		ID:          154587,
		Title:       AnimeTitle{Romaji: "Sousou no Frieren", English: "Frieren: Beyond Journey's End", Native: "葬送のフリーレン"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "The adventure is over but life goes on for an elf mage just beginning to learn what living is all about. The elf mage Frieren and her courageous fellow adventurers have defeated the Demon King and brought peace to the land.",
		Season:      "FALL",
		SeasonYear:  2023,
		Episodes:    28,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx154587-n2bOfvZihioS.png",
			Large:      "https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/bx154587-n2bOfvZihioS.png",
			Color:      "#38bdf8",
		},
		BannerImage:  "https://s4.anilist.co/file/anilistcdn/media/anime/banner/154587-i2PStGZ8pnnE.jpg",
		Genres:       []string{"Adventure", "Drama", "Fantasy"},
		AverageScore: 92,
		MeanScore:    93,
		Popularity:   280000,
		Trending:     100,
		Favourites:   25000,
		StartDate:    &AnimeDate{Year: 2023, Month: 9, Day: 29},
		EndDate:      &AnimeDate{Year: 2024, Month: 3, Day: 22},
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 11, Name: "Madhouse"}},
			},
		},
		Characters: &CharacterConnection{
			Edges: []CharacterEdge{
				{Role: "MAIN", Node: CharacterNode{ID: 1, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Frieren", Native: "フリーレン"}}},
				{Role: "MAIN", Node: CharacterNode{ID: 2, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Fern", Native: "フェルン"}}},
				{Role: "MAIN", Node: CharacterNode{ID: 3, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Stark", Native: "シュタルク"}}},
			},
		},
		Relations: &RelationConnection{
			Edges: []RelationEdge{
				{
					RelationType: "SIDE_STORY",
					Node: RelationNode{
						ID:           154588,
						Title:        AnimeTitle{English: "Frieren Mini Anime"},
						Type:         "ANIME",
						Format:       "SPECIAL",
						Status:       "FINISHED",
						AverageScore: 82,
					},
				},
			},
		},
	},
	{
		ID:          16498,
		Title:       AnimeTitle{Romaji: "Shingeki no Kyojin", English: "Attack on Titan", Native: "進撃の巨人"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "Centuries ago, mankind was slaughtered to near extinction by monstrous humanoid creatures called Titans, forcing humans to hide in fear behind enormous concentric walls.",
		Season:      "SPRING",
		SeasonYear:  2013,
		Episodes:    25,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx16498-C6FPmWm59CyP.jpg",
			Large:      "https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/bx16498-C6FPmWm59CyP.jpg",
			Color:      "#e11d48",
		},
		BannerImage:  "https://s4.anilist.co/file/anilistcdn/media/anime/banner/16498-8jpFCOcDmnei.jpg",
		Genres:       []string{"Action", "Drama", "Fantasy", "Mystery"},
		AverageScore: 89,
		MeanScore:    90,
		Popularity:   650000,
		Trending:     95,
		Favourites:   60000,
		StartDate:    &AnimeDate{Year: 2013, Month: 4, Day: 7},
		EndDate:      &AnimeDate{Year: 2013, Month: 9, Day: 29},
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 858, Name: "WIT Studio"}},
			},
		},
		Characters: &CharacterConnection{
			Edges: []CharacterEdge{
				{Role: "MAIN", Node: CharacterNode{ID: 10, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Eren Yeager", Native: "エレン・イェーガー"}}},
				{Role: "MAIN", Node: CharacterNode{ID: 11, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Mikasa Ackerman", Native: "ミカサ・アッカーマン"}}},
				{Role: "MAIN", Node: CharacterNode{ID: 12, Name: struct {
					Full   string `json:"full"`
					Native string `json:"native,omitempty"`
				}{Full: "Levi", Native: "リヴァイ"}}},
			},
		},
	},
	{
		ID:          113415,
		Title:       AnimeTitle{Romaji: "Jujutsu Kaisen", English: "Jujutsu Kaisen", Native: "呪術廻戦"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "A boy swallows a cursed talisman - the finger of a demon - and becomes cursed himself. He enters a shaman's school to be able to locate the demon's other body parts and thus exorcise himself.",
		Season:      "FALL",
		SeasonYear:  2020,
		Episodes:    24,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx113415-bbBWj4pEFBgM.jpg",
			Color:      "#0284c7",
		},
		Genres:       []string{"Action", "Fantasy", "Supernatural"},
		AverageScore: 86,
		MeanScore:    87,
		Popularity:   500000,
		Trending:     90,
		Favourites:   35000,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 569, Name: "MAPPA"}},
			},
		},
	},
	{
		ID:          101922,
		Title:       AnimeTitle{Romaji: "Kimetsu no Yaiba", English: "Demon Slayer: Kimetsu no Yaiba", Native: "鬼滅の刃"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "It is the Taisho Period in Japan. Tanjiro, a kindhearted boy who sells charcoal for a living, finds his family slaughtered by a demon. To make matters worse, his younger sister Nezuko, the sole survivor, has been transformed into a demon.",
		Season:      "SPRING",
		SeasonYear:  2019,
		Episodes:    26,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx101922-PEn1CTDYxwa2.jpg",
			Color:      "#881337",
		},
		Genres:       []string{"Action", "Adventure", "Fantasy"},
		AverageScore: 85,
		Popularity:   600000,
		Trending:     88,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 43, Name: "ufotable"}},
			},
		},
	},
	{
		ID:          120377,
		Title:       AnimeTitle{Romaji: "Cyberpunk: Edgerunners", English: "Cyberpunk: Edgerunners", Native: "サイバーパンク エッジランナーズ"},
		Format:      "ONA",
		Status:      "FINISHED",
		Description: "A street kid trying to survive in a technology and body modification-obsessed city of the future. Having everything to lose, he chooses to stay alive by becoming an edgerunner: a mercenary outlaw also known as a cyberpunk.",
		Season:      "SUMMER",
		SeasonYear:  2022,
		Episodes:    10,
		Duration:    25,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx120377-pPJCp04tZf8a.jpg",
			Color:      "#facc15",
		},
		Genres:       []string{"Action", "Sci-Fi"},
		AverageScore: 86,
		Popularity:   310000,
		Trending:     85,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 803, Name: "Trigger"}},
			},
		},
	},
	{
		ID:          130003,
		Title:       AnimeTitle{Romaji: "Bocchi the Rock!", English: "Bocchi the Rock!", Native: "ぼっち・ざ・ろっく！"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "Hitori Gotoh, 'Bocchi-chan,' is a girl who's so introverted and shy that she always starts her conversations with 'Ah...' During her middle school years, she started playing the guitar, wanting to join a band because she thought it could be an opportunity for even someone shy like her to also shine.",
		Season:      "FALL",
		SeasonYear:  2022,
		Episodes:    12,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx130003-5A2nQafScGQo.jpg",
			Color:      "#f43f5e",
		},
		Genres:       []string{"Comedy", "Music", "Slice of Life"},
		AverageScore: 88,
		Popularity:   220000,
		Trending:     82,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 1835, Name: "CloverWorks"}},
			},
		},
	},
	{
		ID:          9253,
		Title:       AnimeTitle{Romaji: "Steins;Gate", English: "Steins;Gate", Native: "STEINS;GATE"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "The self-proclaimed mad scientist Rintarou Okabe rents out a room in a rickety old building in Akihabara, where he invents 'future gadgets' with fellow lab members Mayuri Shiina and Hashida Itaru.",
		Season:      "SPRING",
		SeasonYear:  2011,
		Episodes:    24,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx9253-7pdcO9n4tE9s.png",
			Color:      "#64748b",
		},
		Genres:       []string{"Drama", "Sci-Fi", "Suspense"},
		AverageScore: 90,
		Popularity:   420000,
		Trending:     80,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 314, Name: "White Fox"}},
			},
		},
	},
	{
		ID:          1535,
		Title:       AnimeTitle{Romaji: "Death Note", English: "Death Note", Native: "DEATH NOTE"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "A shinigami, as a god of death, can kill any person—provided they see their victim's face and write their victim's name in a notebook called a Death Note. One day, Ryuk drops his Death Note into the human realm.",
		Season:      "FALL",
		SeasonYear:  2006,
		Episodes:    37,
		Duration:    23,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx1535-lawC0OQ9fu9x.jpg",
			Color:      "#070a13",
		},
		Genres:       []string{"Mystery", "Psychological", "Supernatural", "Suspense"},
		AverageScore: 84,
		Popularity:   700000,
		Trending:     75,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 11, Name: "Madhouse"}},
			},
		},
	},
	{
		ID:          5114,
		Title:       AnimeTitle{Romaji: "Hagane no Renkinjutsushi", English: "Fullmetal Alchemist: Brotherhood", Native: "鋼の錬金術師"},
		Format:      "TV",
		Status:      "FINISHED",
		Description: "After a horrific alchemy experiment goes wrong in the Elric household, brothers Edward and Alphonse are left in a catastrophic new reality. Ignoring the alchemical principle of equivalent exchange, the boys attempted human transmutation.",
		Season:      "SPRING",
		SeasonYear:  2009,
		Episodes:    64,
		Duration:    24,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx5114-1oD5sujlBnxO.jpg",
			Color:      "#ea580c",
		},
		Genres:       []string{"Action", "Adventure", "Drama", "Fantasy"},
		AverageScore: 91,
		Popularity:   620000,
		Trending:     78,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 4, Name: "BONES"}},
			},
		},
	},
	{
		ID:          199,
		Title:       AnimeTitle{Romaji: "Sen to Chihiro no Kamikakushi", English: "Spirited Away", Native: "千と千尋の神隠し"},
		Format:      "MOVIE",
		Status:      "FINISHED",
		Description: "Stubborn, spoiled, and naïve, 10-year-old Chihiro Ogino is less than pleased when she and her parents discover an abandoned amusement park on the way to their new house.",
		Season:      "SUMMER",
		SeasonYear:  2001,
		Episodes:    1,
		Duration:    125,
		CoverImage: AnimeCoverImage{
			ExtraLarge: "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx199-aA1pUfF6b7hS.png",
			Color:      "#10b981",
		},
		Genres:       []string{"Adventure", "Supernatural"},
		AverageScore: 87,
		Popularity:   480000,
		Trending:     72,
		Studios: &StudioConnection{
			Edges: []StudioEdge{
				{IsMain: true, Node: StudioNode{ID: 21, Name: "Studio Ghibli"}},
			},
		},
	},
}

// FallbackTrending returns catalog items sorted by Trending DESC.
func FallbackTrending(page, perPage int) *PageResult {
	items := make([]AnimeMedia, len(SeedCatalog))
	copy(items, SeedCatalog)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Trending > items[j].Trending
	})
	return paginateItems(items, page, perPage)
}

// FallbackPopular returns catalog items sorted by Popularity DESC.
func FallbackPopular(page, perPage int) *PageResult {
	items := make([]AnimeMedia, len(SeedCatalog))
	copy(items, SeedCatalog)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Popularity > items[j].Popularity
	})
	return paginateItems(items, page, perPage)
}

// FallbackSearch filters catalog items matching query in titles or genres.
func FallbackSearch(query string, page, perPage int) *PageResult {
	q := strings.ToLower(strings.TrimSpace(query))
	var matched []AnimeMedia
	for _, item := range SeedCatalog {
		if q == "" ||
			strings.Contains(strings.ToLower(item.Title.English), q) ||
			strings.Contains(strings.ToLower(item.Title.Romaji), q) ||
			strings.Contains(strings.ToLower(item.Title.Native), q) {
			matched = append(matched, item)
			continue
		}
		for _, g := range item.Genres {
			if strings.Contains(strings.ToLower(g), q) {
				matched = append(matched, item)
				break
			}
		}
	}
	return paginateItems(matched, page, perPage)
}

// FallbackDetail looks up an item by ID from the seed catalog.
func FallbackDetail(id int) (*AnimeMedia, error) {
	for _, item := range SeedCatalog {
		if item.ID == id {
			c := item
			return &c, nil
		}
	}
	return nil, ErrAnimeNotFound
}

func paginateItems(items []AnimeMedia, page, perPage int) *PageResult {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 20
	}

	total := len(items)
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}

	start := (page - 1) * perPage
	if start >= total {
		return &PageResult{
			PageInfo: PageInfo{
				Total:       total,
				PerPage:     perPage,
				CurrentPage: page,
				LastPage:    lastPage,
				HasNextPage: false,
			},
			Items: []AnimeMedia{},
		}
	}

	end := start + perPage
	if end > total {
		end = total
	}

	return &PageResult{
		PageInfo: PageInfo{
			Total:       total,
			PerPage:     perPage,
			CurrentPage: page,
			LastPage:    lastPage,
			HasNextPage: page < lastPage,
		},
		Items: items[start:end],
	}
}
