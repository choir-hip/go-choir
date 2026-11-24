-- how far back the paper's data goes: the earliest capture and story, and counts by month
SELECT 'captures', to_char(date_trunc('month', fetched_at),'YYYY-MM'), count(*), min(fetched_at) FROM proj_capture_index
  WHERE owner_id='e5962310-8767-4624-87e5-a9b93a4e1db1' AND computer_id='computer-1355b83bced1613eb048ee28d66688a8' GROUP BY 1,2 ORDER BY 2;
SELECT 'stories', to_char(date_trunc('month', created_at),'YYYY-MM'), count(*), min(created_at) FROM og_objects
  WHERE object_kind='choir.publication' AND NOT tombstone AND metadata::json->>'publish_mode'='auto' GROUP BY 1,2 ORDER BY 2;
