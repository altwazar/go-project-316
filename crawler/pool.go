package crawler

import (
	"context"
	"log"
	"sync"
	"time"
)

// pool - структура с пулом воркеров и настройками
type pool struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	opts                 Options
	taskChan             chan task
	linkChecksInProgress map[string]int
	getPagesInProgress   map[string]int
	linkStatuses         map[string]LinkStatus
	pages                []Page
	mu                   sync.RWMutex
	tasksWg              sync.WaitGroup
	workersWg            sync.WaitGroup
	doneChan             chan struct{}
	rateLimiter          *rateLimiter
}

func newPool(ctx context.Context, opts Options) *pool {
	if _, err := normalizeURL(opts.URL); err != nil {
		log.Fatal("Ошибка с корневым url: ", err)
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	taskChanSize := opts.Concurrency * 10
	taskChanSize = max(taskChanSize, 100)
	return &pool{
		ctx:                  ctxWithCancel,
		cancel:               cancel,
		opts:                 opts,
		taskChan:             make(chan task, taskChanSize),
		linkChecksInProgress: make(map[string]int),
		getPagesInProgress:   make(map[string]int),
		linkStatuses:         make(map[string]LinkStatus),
		pages:                []Page{},
		tasksWg:              sync.WaitGroup{},
		workersWg:            sync.WaitGroup{},
		doneChan:             make(chan struct{}),
		rateLimiter:          newRateLimiter(opts.RPS, opts.Delay),
	}
}

func (p *pool) start() {
	p.addTask(newTask(p.opts.URL, getPageTask, 1))

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

// parseResult - формирование отчёта
func parseResult(p *pool) *AnalyzeLinkResponse {
	for i := range p.pages {
		for _, link := range p.pages[i].Links {
			if l, ok := p.linkStatuses[link]; ok && (l.Status >= 400 || l.Error != "") {
				p.pages[i].BrokenLinks = append(p.pages[i].BrokenLinks, l)
			}
		}
	}
	return newAnalyzeResponse(p.opts.URL, p.opts.Depth, p.pages)
}

func newAnalyzeResponse(rootURL string, depth int, pages []Page) *AnalyzeLinkResponse {
	return &AnalyzeLinkResponse{
		RootURL:     rootURL,
		Depth:       depth,
		GeneratedAt: time.Now().UTC().Truncate(time.Second),
		Pages:       pages,
	}
}
