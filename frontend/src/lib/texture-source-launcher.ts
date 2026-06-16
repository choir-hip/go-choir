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
  const contentId = entity?.target?.content_id || '';
  const title = sourceEntityTitle(entity);
  const entityId = sourceEntityID(entity);
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
      mediaType: entity?.kind === 'youtube_video' ? 'video/youtube' : '',
      appHint: appId,
      sourceEntity: entity,
      sourceEntityId: entityId,
      sourceOpenPlan: openPlan,
      sourceReaderMode: !!openPlan.readerMode,
      allowLiveImport: !!openPlan.liveOriginal,
      sourceServiceItemId: entity?.target?.item_id || '',
      publishedRoutePath: entity?.publication_route_path || '',
      publishedGuest: !!entity?.publication_route_path,
      singletonKey: entityId ? `source:${entityId}` : '',
    },
  };
}
