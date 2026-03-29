package web

import (
	"github.com/xxxsen/yamdc/internal/job"
	"github.com/xxxsen/yamdc/internal/medialib"
	"github.com/xxxsen/yamdc/internal/numbercleaner"
	phandler "github.com/xxxsen/yamdc/internal/processor/handler"
	"github.com/xxxsen/yamdc/internal/repository"
	"github.com/xxxsen/yamdc/internal/scanner"
	"github.com/xxxsen/yamdc/internal/searcher"
	"github.com/xxxsen/yamdc/internal/store"
)

type API struct {
	jobRepo  *repository.JobRepository
	scanner  *scanner.Service
	jobSvc   *job.Service
	saveDir  string
	media    *medialib.Service
	store    store.IStorage
	cleaner  numbercleaner.Cleaner
	debugger *searcher.Debugger
	handlers *phandler.Debugger
}

func NewAPI(jobRepo *repository.JobRepository, scanner *scanner.Service, jobSvc *job.Service, saveDir string, media *medialib.Service, storage store.IStorage, cleaner numbercleaner.Cleaner, debugger *searcher.Debugger, handlers *phandler.Debugger) *API {
	return &API{jobRepo: jobRepo, scanner: scanner, jobSvc: jobSvc, saveDir: saveDir, media: media, store: storage, cleaner: cleaner, debugger: debugger, handlers: handlers}
}
