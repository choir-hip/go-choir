import {
  sourceEntityID,
  sourceEntityOpenAppID,
  sourceEntityOpenPlan,
  sourceEntityTargetURL,
  sourceEntityTitle,
} from './texture-source-renderer';

export function sourceEntityLaunchPayload(entity: any): any | null {
  if (!entity) return null;
  const openPlan = sourceEntityOpenPlan(entity);
  const appId = openPlan.appId || sourceEntityOpenAppID(entity);
  const sourceUrl = sourceEntityTargetURL(entity);
  const targetKind = String(entity?.target?.target_kind || entity?.target?.kind || '').trim();
  const contentId = entity?.target?.content_id || (targetKind === 'content_item' ? entity?.target?.id : '') || '';
  const title = sourceEntityTitle(entity);
  const entityId = sourceEntityID(entity);
  const sourceKind = String(entity?.kind || entity?.target?.kind || '').trim();
  return {
    appId,
    appName: title || appId,
    icon: '',
    appContext: {
      windowTitle: title,
      title,
      sourceUrl,
      contentId,
      content_id: contentId,
      mediaType: sourceKind === 'youtube_video' || sourceKind === 'video' ? 'video/youtube' : '',
      appHint: appId,
      sourceEntity: entity,
      sourceEntityId: entityId,
      sourceOpenPlan: openPlan,
      sourceReaderMode: !!openPlan.readerMode,
      allowLiveImport: !!openPlan.liveOriginal,
      sourceServiceItemId: entity?.target?.item_id || (targetKind === 'source_service_item' ? entity?.target?.id : '') || '',
      publishedRoutePath: entity?.publication_route_path || '',
      publishedGuest: !!entity?.publication_route_path,
      singletonKey: entityId ? `source:${entityId}` : '',
    },
  };
}
