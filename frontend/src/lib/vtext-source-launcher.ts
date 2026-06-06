import {
  sourceEntityID,
  sourceEntityOpenAppID,
  sourceEntityTargetURL,
  sourceEntityTitle,
} from './vtext-source-renderer';

export function sourceEntityLaunchPayload(entity: any): any | null {
  if (!entity) return null;
  const appId = sourceEntityOpenAppID(entity);
  const sourceUrl = sourceEntityTargetURL(entity);
  const contentId = entity?.target?.content_id || '';
  const title = sourceEntityTitle(entity);
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
      sourceEntityId: sourceEntityID(entity),
      sourceServiceItemId: entity?.target?.item_id || '',
      publishedRoutePath: entity?.publication_route_path || '',
      publishedGuest: !!entity?.publication_route_path,
      allowMultiple: true,
    },
  };
}
