#!/bin/sh
# Exact never creates provisioning profiles. This builds a stub app with the chosen development app's bundle id (AR_VARIANT=lab or lab-one; a choice is required: the production id has an App Store profile made by the owner in the developer account, never a development one from here) under TEAM (default the free
# team; the paid team is TEAM=S47GERGJZ3) so Xcode (-allowProvisioningUpdates) installs a development profile that Exact then finds.
# The stub's compile may fail after the profile is made; that is fine.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
TEAM="${TEAM:-QSGU6VRJL6}"
case "$("$HERE/app-identity.sh" variant)" in
  ''|dev) echo "make-profile: name the development app: AR_VARIANT=lab or AR_VARIANT=lab-one (the production id, which AR_VARIANT=dev is, gets no development profile from here: it is made through the App Store Connect API, docs/reference/exact/build-and-ship.md)" >&2; exit 1 ;;
esac
ID="$("$HERE/app-identity.sh" id)"   # AR_VARIANT / AR_APP_ID choose the app
W="$(mktemp -d)"
cat >"$W/main.swift" <<'EOS'
import UIKit
@main class A: UIResponder, UIApplicationDelegate {}
EOS
cat >"$W/project.yml" <<EOS
name: LabStub
targets:
  LabStub:
    type: application
    platform: iOS
    deploymentTarget: "18.0"
    sources: [main.swift]
    settings:
      base:
        PRODUCT_BUNDLE_IDENTIFIER: $ID
        DEVELOPMENT_TEAM: $TEAM
        CODE_SIGN_STYLE: Automatic
        GENERATE_INFOPLIST_FILE: YES
EOS
cd "$W"
xcodegen generate
"$HERE/xe.sh" /usr/bin/xcodebuild -project LabStub.xcodeproj -scheme LabStub -destination 'generic/platform=iOS' \
  -allowProvisioningUpdates -derivedDataPath dd build || true
ls "$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles"
