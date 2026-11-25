# Privacy policy: published

The owner approved the draft on 2026-11-24 ("yes, i approved the new privacy
policy already lol."). The policy is published. There is one text:
`frontend/public/legal/privacy-policy.md`, served at choir.news/privacy and
fetched by the app. Edit that file, not a copy here. Earlier versions are in
the public git history of that file.

Sentences that depend on work not on main when it was published (2026-11-24):

- Section 9 (deleting your account in the app and on choir.news, the 72 hour
  hold, the cancel until the removal begins, what stays, the backup window and
  the list of deletions a restore carries out again): written with lane
  `account-delete` (landed as `acctdel-land`) and true once that lane is
  deployed and its drill has passed. Until then the published text said
  deletion was by email. The App Store build must not ship before it is
  deployed.
- Section 8 (server logs): states no deletion time. When lane `log-retention`
  is deployed and proved on the host (it is black: it deletes files on Node B
  and needs the owner's word), change the line to its 30 day statement.

Sources for each statement: [data-inventory.md](data-inventory.md).
