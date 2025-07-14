package models

type Provider interface {
	Init()
	Search          ( query      string )                          ( []Series, error)
	GetEpisodes     ( animeModel Series,  start   uint, end uint ) ( []Episode, error)
	DownloadEpisode ( episode    Episode, rootDir string )         ( string, error)
}
