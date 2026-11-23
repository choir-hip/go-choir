#!/bin/sh
# Exact never creates provisioning profiles. This builds a stub app with the chosen app's bundle id (AR_VARIANT=lab or lab-one; none is the production id) under TEAM (default the free
# team; the paid team is TEAM=S47GERGJZ3) so Xcode (-allowProvisioningUpdates) installs a development profile that Exact then finds.
# The stub's compile may fail after the profile is made; that is fine.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
TEAM="${TEAM:-QSGU6VRJL6}"
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
