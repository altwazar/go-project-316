// Package crawler - основная логика краулера
package crawler

import (
	"context"
	"sort"
	"sync"
	"time"

	"code/internal/fetcher"
	"code/internal/models"
	"code/internal/urlutil"
)

type pool struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	opts                  models.Options
	taskChan              chan task
	linkChecksInProgress  map[string]int
	getPagesInProgress    map[string]int
	assetChecksInProgress map[string]int
	assetsStatuses        map[string]models.Asset
	linkStatuses          map[string]models.LinkStatus
	pages                 []models.Page
	mu                    sync.RWMutex
	tasksWg               sync.WaitGroup
	workersWg             sync.WaitGroup
	doneChan              chan struct{}
	rateLimiter           *fetcher.RateLimiter
}

func newPool(ctx context.Context, opts models.Options) (*pool, error) {
	if _, err := urlutil.NormalizeURL(opts.URL); err != nil {
		return &pool{}, err
	}
	ctxWithCancel, cancel := context.WithCancel(ctx)

	taskChanSize := opts.Concurrency * 10
	taskChanSize = max(taskChanSize, 100)
	return &pool{
		ctx:                   ctxWithCancel,
		cancel:                cancel,
		opts:                  opts,
		taskChan:              make(chan task, taskChanSize),
		linkChecksInProgress:  make(map[string]int),
		getPagesInProgress:    make(map[string]int),
		assetChecksInProgress: make(map[string]int),
		assetsStatuses:        make(map[string]models.Asset),
		linkStatuses:          make(map[string]models.LinkStatus),
		pages:                 []models.Page{},
		tasksWg:               sync.WaitGroup{},
		workersWg:             sync.WaitGroup{},
		doneChan:              make(chan struct{}),
		rateLimiter:           fetcher.NewRateLimiter(opts.RPS, opts.Delay),
	}, nil
}

func (p *pool) start() {
	p.addTask(newPageTask(p.opts.URL, 0))

	for i := 0; i < p.opts.Concurrency; i++ {
		p.workersWg.Add(1)
		go worker(p)
	}

	go p.monitorTasks()
}

func (p *pool) wait() {
	<-p.doneChan
}

func (p *pool) close() {
	p.cancel()
}

func (p *pool) addTask(t *task) {
	p.tasksWg.Add(1)
	select {
	case p.taskChan <- *t:
	case <-p.ctx.Done():
		p.tasksWg.Done()
	}
}

func (p *pool) taskDone() {
	p.tasksWg.Done()
}

func (p *pool) monitorTasks() {
	p.tasksWg.Wait()
	close(p.taskChan)
	p.workersWg.Wait()
	close(p.doneChan)
}

func worker(p *pool) {
	defer p.workersWg.Done()
	for task := range p.taskChan {
		if err := p.rateLimiter.Wait(p.ctx); err != nil {
			return
		}
		task.execute(p)
	}
}

func parseResult(p *pool) *models.Report {
	for i := range p.pages {
		p.processPageBrokenLinks(i)
		p.processPageAssets(i)
	}

	sort.Slice(p.pages, func(i, j int) bool {
		return p.pages[i].URL < p.pages[j].URL
	})

	return newAnalyzeResponse(p.opts.URL, p.opts.Depth, p.pages)
}

func (p *pool) processPageBrokenLinks(pageIndex int) {
	for _, link := range p.pages[pageIndex].Links {
		if p.isBrokenLink(link) {
			p.pages[pageIndex].BrokenLinks = append(p.pages[pageIndex].BrokenLinks, p.linkStatuses[link])
		}
	}
}

func (p *pool) isBrokenLink(link string) bool {
	status, exists := p.linkStatuses[link]
	if !exists {
		return false
	}
	return status.StatusCode >= 400 || status.Error != ""
}

func (p *pool) processPageAssets(pageIndex int) {
	for assetIdx := range p.pages[pageIndex].Assets {
		p.updateAssetWithStatus(pageIndex, assetIdx)
	}

	sort.Slice(p.pages[pageIndex].Assets, func(i, j int) bool {
		if p.pages[pageIndex].Assets[i].Type != p.pages[pageIndex].Assets[j].Type {
			return p.pages[pageIndex].Assets[i].Type < p.pages[pageIndex].Assets[j].Type
		}
		return p.pages[pageIndex].Assets[i].URL < p.pages[pageIndex].Assets[j].URL
	})
}

func (p *pool) updateAssetWithStatus(pageIndex, assetIndex int) {
	assetURL := p.pages[pageIndex].Assets[assetIndex].URL
	normalizedURL, _ := urlutil.NormalizeURL(assetURL)

	if statusAsset, exists := p.assetsStatuses[normalizedURL]; exists {
		p.pages[pageIndex].Assets[assetIndex] = statusAsset
	}
}

func newAnalyzeResponse(rootURL string, depth int, pages []models.Page) *models.Report {
	return &models.Report{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
}
