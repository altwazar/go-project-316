package crawler

import (
	"context"
	"sync"
	"time"
)

// pool - структура с пулом воркеров и настройками
type pool struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	opts                  Options
	taskChan              chan task
	linkChecksInProgress  map[string]int
	getPagesInProgress    map[string]int
	assetChecksInProgress map[string]int
	assetsStatuses        map[string]Asset
	linkStatuses          map[string]LinkStatus
	pages                 []Page
	mu                    sync.RWMutex
	tasksWg               sync.WaitGroup
	workersWg             sync.WaitGroup
	doneChan              chan struct{}
	rateLimiter           *rateLimiter
}

func newPool(ctx context.Context, opts Options) (*pool, error) {
	if _, err := normalizeURL(opts.URL); err != nil {
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
		assetsStatuses:        make(map[string]Asset),
		linkStatuses:          make(map[string]LinkStatus),
		pages:                 []Page{},
		tasksWg:               sync.WaitGroup{},
		workersWg:             sync.WaitGroup{},
		doneChan:              make(chan struct{}),
		rateLimiter:           newRateLimiter(opts.RPS, opts.Delay),
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
		if err := p.rateLimiter.wait(p.ctx); err != nil {
			return
		}
		task.execute(p)
	}
}

// parseResult - формирование отчёта (сложность 3)
func parseResult(p *pool) *Report {
	for i := range p.pages {
		p.processPageBrokenLinks(i)
		p.processPageAssets(i)
	}
	return newAnalyzeResponse(p.opts.URL, p.opts.Depth, p.pages)
}

// processPageBrokenLinks - обработка битых ссылок на странице
func (p *pool) processPageBrokenLinks(pageIndex int) {
	for _, link := range p.pages[pageIndex].Links {
		if p.isBrokenLink(link) {
			p.pages[pageIndex].BrokenLinks = append(p.pages[pageIndex].BrokenLinks, p.linkStatuses[link])
		}
	}
}

// isBrokenLink - проверка является ли ссылка битой
func (p *pool) isBrokenLink(link string) bool {
	status, exists := p.linkStatuses[link]
	if !exists {
		return false
	}
	return status.StatusCode >= 400 || status.Error != ""
}

// processPageAssets - обработка ассетов на странице
func (p *pool) processPageAssets(pageIndex int) {
	for assetIdx := range p.pages[pageIndex].Assets {
		p.updateAssetWithStatus(pageIndex, assetIdx)
	}
}

// updateAssetWithStatus - обновление ассета статусом
func (p *pool) updateAssetWithStatus(pageIndex, assetIndex int) {
	assetURL := p.pages[pageIndex].Assets[assetIndex].URL
	normalizedURL, _ := normalizeURL(assetURL)

	if statusAsset, exists := p.assetsStatuses[normalizedURL]; exists {
		p.pages[pageIndex].Assets[assetIndex] = statusAsset
	}
}

func newAnalyzeResponse(rootURL string, depth int, pages []Page) *Report {
	return &Report{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
}
