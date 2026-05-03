// Package crawler - основная логика краулера
package crawler

import (
	"code/internal/fetcher"
	"code/internal/models"
	"code/internal/urlutil"
)

type taskType int

const (
	getPageTask taskType = iota
	checkLinkTask
	checkAssetTask
)

type task struct {
	url       string
	taskType  taskType
	assetType *models.AssetType
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

func newAssetCheckTask(url string, assetType models.AssetType) *task {
	normalizedURL, err := urlutil.NormalizeURL(url)
	if err != nil {
		normalizedURL = url
	}
	return &task{
		url:       normalizedURL,
		taskType:  checkAssetTask,
		assetType: &assetType,
	}
}

func (t *task) execute(p *pool) {
	defer p.taskDone()
	u, err := urlutil.NormalizeURL(t.url)
	switch t.taskType {
	case getPageTask:
		t.executeGetPage(p, u, err)
	case checkLinkTask:
		t.executeCheckLink(p, u, err)
	case checkAssetTask:
		t.executeCheckAsset(p, u, err)
	}
}

func (t *task) executeCheckAsset(p *pool, u string, err error) {
	p.mu.Lock()
	_, inProgress := p.assetChecksInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return
	}
	p.assetChecksInProgress[u] = 1
	p.mu.Unlock()

	asset := models.Asset{URL: t.url, Type: *t.assetType}
	if err != nil {
		asset.Error = err.Error()
	} else {
		checkedAsset, err := fetcher.CheckAsset(p.ctx, u, *t.assetType, p.opts.HTTPClient, p.opts.UserAgent)
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

func (t *task) executeGetPage(p *pool, u string, err error) {
	if t.shouldSkipDuplicatePage(p, u) {
		return
	}
	pg := t.fetchPage(p, u, err)
	p.mu.Lock()
	p.pages = append(p.pages, pg)
	p.mu.Unlock()
}

func (t *task) shouldSkipDuplicatePage(p *pool, u string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, inProgress := p.getPagesInProgress[u]
	if inProgress {
		return true
	}
	p.getPagesInProgress[u] = 1
	return false
}

func (t *task) fetchPage(p *pool, u string, err error) models.Page {
	if err != nil {
		return models.Page{URL: t.url, Error: err.Error()}
	}
	pg := fetcher.FetchPageWithRetries(p.ctx, u, t.depth, p.opts)
	t.scheduleChildTasks(p, &pg)
	return pg
}

func (t *task) scheduleChildTasks(p *pool, pg *models.Page) {
	t.scheduleLinkTasks(p, pg)
	t.scheduleAssetTasks(p, pg)
}

func (t *task) scheduleLinkTasks(p *pool, pg *models.Page) {
	shouldCrawlDeeper := p.opts.Depth > t.depth+1
	for _, ln := range pg.Links {
		if shouldCrawlDeeper && urlutil.IsSameDomain(p.opts.URL, ln) {
			p.addTask(newPageTask(ln, t.depth+1))
		} else {
			p.addTask(newLinkCheckTask(ln, t.depth+1))
		}
	}
}

func (t *task) scheduleAssetTasks(p *pool, pg *models.Page) {
	for _, asset := range pg.Assets {
		p.addTask(newAssetCheckTask(asset.URL, asset.Type))
	}
}

func (t *task) executeCheckLink(p *pool, u string, err error) {
	p.mu.Lock()
	_, inProgress := p.linkChecksInProgress[u]
	if inProgress {
		p.mu.Unlock()
		return
	}
	p.linkChecksInProgress[u] = 1
	p.mu.Unlock()

	var ln models.LinkStatus
	if err != nil {
		ln = models.LinkStatus{URL: t.url, Error: err.Error()}
	} else {
		s, err := fetcher.CheckLinkStatus(p.ctx, u, p.opts.HTTPClient, p.opts.UserAgent)
		if err == nil {
			ln = models.LinkStatus{URL: t.url, StatusCode: s}
		} else {
			ln = models.LinkStatus{URL: t.url, Error: err.Error()}
		}
	}

	p.mu.Lock()
	p.linkStatuses[u] = ln
	p.mu.Unlock()
}
