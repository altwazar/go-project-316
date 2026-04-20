package crawler

// taskType - тип задачи
type taskType int

const (
	getPageTask taskType = iota
	checkLinkTask
)

// task - структура задачи
type task struct {
	url      string
	taskType taskType
	depth    int
}

// newTask - конструктор
func newTask(url string, t taskType, depth int) *task {
	return &task{
		url:      url,
		taskType: t,
		depth:    depth,
	}
}

// execute - выполнение задачи
func (t *task) execute(p *pool) {
	defer p.taskDone()

	u, err := normalizeURL(t.url)

	if t.taskType == getPageTask {
		t.executeGetPage(p, u, err)
	} else {
		t.executeCheckLink(p, u, err)
	}
}

// executeGetPage - выполнение задачи получения данных со страницы
func (t *task) executeGetPage(p *pool, u string, err error) {
	p.mu.Lock()
	_, inProgress := p.getPagesInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return
	}
	p.getPagesInProgress[u] = 1
	p.mu.Unlock()

	pg := Page{}
	if err != nil {
		pg = Page{URL: t.url, Error: err.Error()}
	} else {
		pg = getPageWithRetries(p.ctx, u, t.depth, p.opts)
		if p.opts.Depth > t.depth {
			for _, ln := range pg.Links {
				if isSameDomain(p.opts.URL, ln) {
					p.addTask(newTask(ln, getPageTask, t.depth+1))
				} else {
					p.addTask(newTask(ln, checkLinkTask, t.depth+1))
				}
			}
		} else {
			for _, ln := range pg.Links {
				p.addTask(newTask(ln, checkLinkTask, t.depth+1))
			}
		}
	}

	p.mu.Lock()
	p.pages = append(p.pages, pg)
	p.mu.Unlock()
}

// executeCheckLink - выполнение задачи проверки проверки ссылки
func (t *task) executeCheckLink(p *pool, u string, err error) {
	p.mu.Lock()
	_, inProgress := p.linkChecksInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return
	}
	p.linkChecksInProgress[u] = 1
	p.mu.Unlock()

	ln := LinkStatus{}
	if err != nil {
		ln = LinkStatus{URL: t.url, Error: err.Error()}
	} else {
		s, err := checkLinkStatus(p.ctx, u, p.opts.HTTPClient)
		if err == nil {
			ln = LinkStatus{URL: t.url, Status: s}
		} else {
			ln = LinkStatus{URL: t.url, Error: err.Error()}
		}
	}

	p.mu.Lock()
	p.linkStatuses[u] = ln
	p.mu.Unlock()
}
