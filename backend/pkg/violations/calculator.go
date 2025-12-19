package violations

import (
	"context"
	"time"
)

type Calculator struct {
	repo    *Repository
	matcher *Matcher
}

func NewCalculator(repo *Repository, matcher *Matcher) *Calculator {
	return &Calculator{
		repo:    repo,
		matcher: matcher,
	}
}

func (c *Calculator) CalculateForContent(ctx context.Context, content ContentInfo) (*ContentStats, error) {
	matches, err := c.matcher.FindAllMatches(ctx, content)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		if err := c.repo.DeleteByContentID(ctx, content.ID); err != nil {
			return nil, err
		}
		return &ContentStats{
			ContentID:       content.ID,
			ViolationsCount: 0,
			SitesCount:      0,
		}, nil
	}

	now := time.Now()
	violations := make([]Violation, len(matches))
	pageIDs := make([]string, len(matches))
	siteSet := make(map[string]struct{})

	for i, match := range matches {
		violations[i] = Violation{
			ContentID: content.ID,
			SiteID:    match.SiteID,
			PageID:    match.PageID,
			PageURL:   match.URL,
			PageTitle: match.Title,
			MatchType: match.MatchType,
			FoundAt:   now,
		}
		pageIDs[i] = match.PageID
		siteSet[match.SiteID] = struct{}{}
	}

	if err := c.repo.UpsertMany(ctx, violations); err != nil {
		return nil, err
	}

	if err := c.repo.DeleteNotInPageIDs(ctx, content.ID, pageIDs); err != nil {
		return nil, err
	}

	siteIDs := make([]string, 0, len(siteSet))
	for siteID := range siteSet {
		siteIDs = append(siteIDs, siteID)
	}

	return &ContentStats{
		ContentID:       content.ID,
		ViolationsCount: int64(len(matches)),
		SitesCount:      int64(len(siteIDs)),
		PageIDs:         pageIDs,
		SiteIDs:         siteIDs,
	}, nil
}

func (c *Calculator) CalculateForAllContent(ctx context.Context, contents []ContentInfo) (int64, error) {
	var updated int64
	for _, content := range contents {
		_, err := c.CalculateForContent(ctx, content)
		if err != nil {
			continue
		}
		updated++
	}
	return updated, nil
}

// CalculateForSite обновляет violations только для страниц конкретного сайта
func (c *Calculator) CalculateForSite(ctx context.Context, siteID string, contents []ContentInfo) (int64, error) {
	var updated int64
	now := time.Now()

	for _, content := range contents {
		matches, err := c.matcher.FindAllMatchesForSite(ctx, content, siteID)
		if err != nil {
			continue
		}

		var pageIDs []string
		var violations []Violation

		for _, match := range matches {
			violations = append(violations, Violation{
				ContentID: content.ID,
				SiteID:    match.SiteID,
				PageID:    match.PageID,
				PageURL:   match.URL,
				PageTitle: match.Title,
				MatchType: match.MatchType,
				FoundAt:   now,
			})
			pageIDs = append(pageIDs, match.PageID)
		}

		if len(violations) > 0 {
			if err := c.repo.UpsertMany(ctx, violations); err != nil {
				continue
			}
		}

		if err := c.repo.DeleteByContentAndSiteNotInPageIDs(ctx, content.ID, siteID, pageIDs); err != nil {
			continue
		}

		updated += int64(len(violations))
	}

	return updated, nil
}
