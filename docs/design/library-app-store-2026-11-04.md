# The Library as an app store (2026-11-04)

Owner review on a phone, 2026-11-04: "For an app to get into the library, it needs to have screenshots. It needs to be installable. We shouldn't be showing all this metadata ... It should show the email address of the account that made it. And you need to be able to see the screenshots ... Remember, this is an app store." Then: one level, not two ("as soon as you look at that app, you should be able to see the information about it"); precommitments stay, readable; metadata hidden; no Base image tab.

## Rules

1. **Screenshots gate the listing.** A release with no screenshot is refused at publish (`screenshot_required`, 422) at three places: the guest release-draft route, the proxy before any write, corpusd. A video alone is not a screenshot. The catalog lists only releases that have one; its history drops the others for every reader but the publisher (who keeps them to withdraw). Nothing is deleted; a direct read of a stored release still answers, and install of one by id is unchanged.
2. **Publisher identity is an email.** The proxy resolves the publisher's owner id to the account email from the auth store it already reads (`internal/proxy/packages_publisher.go`) and adds `publisher_email` and `own` to the catalog, history and release answers. corpusd and the record never hold the address; the field is display-only and nothing acts on it. An owner the store cannot resolve gets no field.
3. **Privacy fact about publishing.** The owner explicitly asked for this: publishing a release makes the account's email address visible to every signed-in user, on the app's card and page, for as long as the release is listed. Withdrawing the release removes it from the store. There is no per-publisher opt-out.
4. **Card and page.** One card per app and maker: thumbnail (first screenshot), name, one-line description (the record's optional `description`, the manifest's display description, checked by the host against the manifest bytes), maker's email, "Updated Nov 4". Tapping opens one page: name, email, description, Install / Update / Installed + Uninstall, a swipeable screenshot strip (tap to enlarge, the video inline), the precommitments in words ("Predicted 88% likely", "Came true"), and a collapsed "Version history" labelled "Version 2, Nov 4". No id, track, computer or metadata panel appears in the user view.

Mutation classes: server and publish gate red; frontend orange. Rollback is a revert (screenshot-less releases reappear in the listing; none were deleted). Rollout is by ring with no switch.

Residual: the display name is the package name (the record carries no title); the older versions' install button installs that version by its own record.
