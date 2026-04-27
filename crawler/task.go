package crawler

// taskType - тип задачи
type taskType int

const (
	getPageTask taskType = iota
	checkLinkTask
	checkAssetTask
)

type task struct {
	url       string
	taskType  taskType
	assetType *AssetType // указатель вместо значения
	depth     int
}

func newPageTask(url string, depth int) *task {
	return &task{
		url:      url,
		taskType: getPageTask,
		depth:    depth,
	}
}

func newLinkCheckTask(url string, depth int) *task {
	return &task{
		url:      url,
		taskType: checkLinkTask,
		depth:    depth,
	}
}

func newAssetCheckTask(url string, assetType AssetType) *task {
	normalizedURL, err := normalizeURL(url)
	if err != nil {
		normalizedURL = url
	}
	return &task{
		url:       normalizedURL,
		taskType:  checkAssetTask,
		assetType: &assetType,
	}
}

// execute - выполнение задачи
func (t *task) execute(p *pool) {
	defer p.taskDone()
	u, err := normalizeURL(t.url)
	switch t.taskType {
	case getPageTask:
		t.executeGetPage(p, u, err)
	case checkLinkTask:
		t.executeCheckLink(p, u, err)
	case checkAssetTask:
		t.executeCheckAsset(p, u, err)
	}
}

// executeCheckAsset - выполнение задачи проверки ассета
func (t *task) executeCheckAsset(p *pool, u string, err error) {
	p.mu.Lock()
	_, inProgress := p.assetChecksInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return
	}
	p.assetChecksInProgress[u] = 1
	p.mu.Unlock()

	asset := Asset{URL: t.url, Type: *t.assetType}
	if err != nil {
		asset.Error = err.Error()
	} else {
		checkedAsset, err := checkAsset(p.ctx, u, *t.assetType, p.opts.HTTPClient)
		if err != nil {
			asset.Error = err.Error()
		} else {
			asset = checkedAsset
		}
	}

	p.mu.Lock()
	p.assetsStatuses[u] = asset
	p.mu.Unlock()
}

// executeGetPage - выполнение задачи получения данных со страницы
func (t *task) executeGetPage(p *pool, u string, err error) {
	if t.shouldSkipDuplicatePage(p, u) {
		return
	}

	pg := t.fetchPage(p, u, err)

	p.mu.Lock()
	p.pages = append(p.pages, pg)
	p.mu.Unlock()
}

// shouldSkipDuplicatePage - проверяет, не выполняется ли уже эта страница
func (t *task) shouldSkipDuplicatePage(p *pool, u string) bool {
	p.mu.Lock()
	_, inProgress := p.getPagesInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return true
	}
	p.getPagesInProgress[u] = 1
	p.mu.Unlock()
	return false
}

// fetchPage - получение страницы и создание подзадач
func (t *task) fetchPage(p *pool, u string, err error) Page {
	if err != nil {
		return Page{URL: t.url, Error: err.Error()}
	}

	pg := getPageWithRetries(p.ctx, u, t.depth, p.opts)
	t.scheduleChildTasks(p, &pg)
	return pg
}

// scheduleChildTasks - создание подзадач на основе полученной страницы
func (t *task) scheduleChildTasks(p *pool, pg *Page) {
	t.scheduleLinkTasks(p, pg)
	t.scheduleAssetTasks(p, pg)
}

// scheduleLinkTasks - создание задач для ссылок
func (t *task) scheduleLinkTasks(p *pool, pg *Page) {
	shouldCrawlDeeper := p.opts.Depth > t.depth

	for _, ln := range pg.Links {
		if shouldCrawlDeeper && isSameDomain(p.opts.URL, ln) {
			p.addTask(newPageTask(ln, t.depth+1))
		} else {
			p.addTask(newLinkCheckTask(ln, t.depth+1))
		}
	}
}

// scheduleAssetTasks - создание задач для ассетов
func (t *task) scheduleAssetTasks(p *pool, pg *Page) {
	for _, asset := range pg.Assets {
		p.addTask(newAssetCheckTask(asset.URL, asset.Type))
	}
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

	var ln LinkStatus
	if err != nil {
		ln = LinkStatus{URL: t.url, Error: err.Error()}
	} else {
		s, err := checkLinkStatus(p.ctx, u, p.opts.HTTPClient)
		if err == nil {
			ln = LinkStatus{URL: t.url, StatusCode: s}
		} else {
			ln = LinkStatus{URL: t.url, Error: err.Error()}
		}
	}

	p.mu.Lock()
	p.linkStatuses[u] = ln
	p.mu.Unlock()
}
