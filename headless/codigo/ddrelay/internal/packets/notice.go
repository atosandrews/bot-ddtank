package packets

func notice(r *reader, fields map[string]any) error {
	kind := r.i32("notice_type")
	fields["notice_type"] = kind
	fields["message"] = r.utf("message")
	if kind == 4 {
		return nil
	}
	if kind == 8 || kind == 10 || kind == 11 || kind == 18 || kind == 13 || kind == 12 {
		fields["zone_id"] = r.i32("zone_id")
	}
	keyed := kind == 1 || kind == 5 || kind == 20 || kind == 10 || kind == 11 || kind == 18
	if r.err != nil || r.offset == len(r.data) {
		return r.err
	}
	count := int(r.u8("link_count"))
	links := make([]map[string]any, 0, count)
	for i := 0; i < count && r.err == nil; i++ {
		link := make(map[string]any)
		link["index"] = r.i32("index")
		link["template_id"] = r.i32("template_id")
		link["item_id"] = r.i32("item_id")
		if keyed {
			link["key"] = r.utf("key")
		}
		links = append(links, link)
	}
	fields["item_links"] = links
	return r.err
}
