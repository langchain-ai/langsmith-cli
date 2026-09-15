package cmd

type offsetPageInfo struct {
	Returned   int    `json:"returned"`
	HasMore    *bool  `json:"has_more"`
	NextOffset *int64 `json:"next_offset"`
}

// A full offset page does not prove another item exists. Preserve that uncertainty
// rather than telling an agent that the response is either complete or truncated.
func describeOffsetPage(count int, limit, offset int64) offsetPageInfo {
	info := offsetPageInfo{Returned: count}
	if int64(count) < limit {
		more := false
		info.HasMore = &more
	} else {
		next := offset + int64(count)
		info.NextOffset = &next
	}
	return info
}

func resultWorkspaceID() *string {
	id := GetWorkspaceID()
	if id == "" {
		return nil
	}
	return &id
}
