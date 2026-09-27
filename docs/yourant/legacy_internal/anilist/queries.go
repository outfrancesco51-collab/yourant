package anilist

// TrendingQuery retrieves currently trending anime ordered by TRENDING_DESC.
const TrendingQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo {
      total
      perPage
      currentPage
      lastPage
      hasNextPage
    }
    media(type: ANIME, sort: TRENDING_DESC, isAdult: false) {
      id
      idMal
      title { romaji english native }
      format
      status
      description
      season
      seasonYear
      episodes
      duration
      coverImage { extraLarge large medium color }
      bannerImage
      genres
      averageScore
      meanScore
      popularity
      trending
      favourites
      startDate { year month day }
      endDate { year month day }
      studios(isMain: true) {
        edges { isMain node { id name } }
      }
      nextAiringEpisode { airingAt timeUntilAiring episode }
    }
  }
}
`

// PopularQuery retrieves all-time popular anime ordered by POPULARITY_DESC.
const PopularQuery = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo {
      total
      perPage
      currentPage
      lastPage
      hasNextPage
    }
    media(type: ANIME, sort: POPULARITY_DESC, isAdult: false) {
      id
      idMal
      title { romaji english native }
      format
      status
      description
      season
      seasonYear
      episodes
      duration
      coverImage { extraLarge large medium color }
      bannerImage
      genres
      averageScore
      popularity
      trending
      startDate { year month day }
      studios(isMain: true) {
        edges { isMain node { id name } }
      }
    }
  }
}
`

// SearchQuery searches anime by title or keywords.
const SearchQuery = `
query ($search: String, $page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo {
      total
      perPage
      currentPage
      lastPage
      hasNextPage
    }
    media(type: ANIME, search: $search, sort: SEARCH_MATCH, isAdult: false) {
      id
      idMal
      title { romaji english native }
      format
      status
      description
      season
      seasonYear
      episodes
      duration
      coverImage { extraLarge large medium color }
      bannerImage
      genres
      averageScore
      popularity
      trending
      startDate { year month day }
      studios(isMain: true) {
        edges { isMain node { id name } }
      }
    }
  }
}
`

// DetailQuery retrieves comprehensive anime details including characters and relations.
const DetailQuery = `
query ($id: Int) {
  Media(id: $id, type: ANIME) {
    id
    idMal
    title { romaji english native }
    format
    status
    description
    season
    seasonYear
    episodes
    duration
    coverImage { extraLarge large medium color }
    bannerImage
    genres
    averageScore
    meanScore
    popularity
    trending
    favourites
    startDate { year month day }
    endDate { year month day }
    studios {
      edges { isMain node { id name } }
    }
    characters(sort: ROLE, perPage: 12) {
      edges {
        role
        node {
          id
          name { full native }
          image { large medium }
        }
      }
    }
    relations {
      edges {
        relationType
        node {
          id
          title { romaji english }
          type
          format
          status
          coverImage { large medium }
          averageScore
        }
      }
    }
    nextAiringEpisode { airingAt timeUntilAiring episode }
  }
}
`
