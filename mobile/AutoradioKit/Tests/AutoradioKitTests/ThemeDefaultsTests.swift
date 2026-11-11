import XCTest
@testable import AutoradioKit

// The default themes: a person who has not chosen one sees Chrome Pastel in a light scheme and Liquid Metal in a dark one (the
// web's defaults, translated from the web presets). A chosen theme always wins and the default is never written as a choice.
// Failure list, written before the code:
//  D1 a light scheme shows something other than Chrome Pastel; D2 a dark one something other than Liquid Metal;
//  D3 an explicit mode is ignored for the default (dark mode on a light phone still shows the light default);
//  D4 a chosen theme is replaced by the default, in either scheme, or after a relaunch;
//  D5 the default is written to the preferences as if chosen, so it can no longer follow the scheme;
//  D6 either theme is refused by the contrast gate, or loses a token the format needs (scrim, onScrim, bar and layer backing);
//  D7 the body face is not the sans the web theme reads in;
//  D8 removing the chosen theme leaves a stale choice instead of returning to the scheme default.

private func fresh() -> UserDefaults { UserDefaults(suiteName: "defaults-\(UUID().uuidString)")! }
private func store(_ d: UserDefaults, system: String = "light") -> ThemeStore {
    ThemeStore(userDir: nil, defaults: d, systemScheme: { system })
}

final class ThemeDefaultsTests: XCTestCase {
    func testLightSchemeShowsChromePastel() {                                  // D1
        let s = store(fresh(), system: "light")
        XCTAssertEqual(s.themeID, "chrome-pastel")
        XCTAssertEqual(s.current().scheme, "light"); XCTAssertEqual(s.current().paper, "#F7FBFE")
        XCTAssertEqual(s.list().first { $0.current }?.id, "chrome-pastel")
    }

    func testDarkSchemeShowsLiquidMetal() {                                    // D2
        let s = store(fresh(), system: "dark")
        XCTAssertEqual(s.themeID, "liquid-metal")
        XCTAssertEqual(s.current().scheme, "dark"); XCTAssertEqual(s.current().paper, "#1C242E")
        XCTAssertEqual(s.cards().first { $0.current }?.id, "default", "no choice: the Default entry is the one in use")
        XCTAssertEqual(s.cards().first?.id, "default", "and it comes first")
        XCTAssertEqual(s.cards().first?.paper, "#1C242E", "painted as the pair shows in the scheme in force")
    }

    func testAnExplicitModeDecidesTheDefault() {                               // D3
        let a = store(fresh(), system: "light"); a.setMode("dark")
        XCTAssertEqual(a.themeID, "liquid-metal"); XCTAssertEqual(a.effectiveScheme(), "dark")
        let b = store(fresh(), system: "dark"); b.setMode("light")
        XCTAssertEqual(b.themeID, "chrome-pastel"); XCTAssertEqual(b.effectiveScheme(), "light")
    }

    func testAChosenThemeWinsInEitherSchemeAndAfterARelaunch() {               // D4
        let d = fresh()
        let s = store(d, system: "dark")
        s.setTheme("depth")
        XCTAssertEqual(s.themeID, "depth"); XCTAssertEqual(s.current().scheme, "dark")
        let again = store(d, system: "light")
        XCTAssertEqual(again.themeID, "depth")
        again.setMode("dark"); XCTAssertEqual(again.themeID, "depth")
    }

    func testTheDefaultIsNotStoredAsAChoice() {                                // D5
        let d = fresh()
        let s = store(d, system: "dark")
        _ = s.current(); _ = s.cards(); _ = s.themeID; s.setMode("light"); _ = s.current()
        XCTAssertNil(d.string(forKey: "AutoradioThemeID"))
        XCTAssertEqual(s.themeID, "chrome-pastel", "and it follows the mode")
    }

    func testBothDefaultsPassTheGateAndCarryEveryToken() {                     // D6
        let s = store(fresh())
        XCTAssertEqual(s.problems, [])
        for (mode, id) in [("light", "chrome-pastel"), ("dark", "liquid-metal")] {
            s.setMode(mode)
            XCTAssertEqual(s.themeID, id)
            let t = s.current()
            XCTAssertEqual(t.scheme, mode)
            XCTAssertFalse(t.scrim.isEmpty); XCTAssertNotEqual(t.scrim, "rgba(0,0,0,.7)", "the theme's own scrim, not the fallback")
            XCTAssertFalse(t.onScrim.isEmpty)
            XCTAssertEqual(t.barBacking, 0.94); XCTAssertEqual(t.layerBacking, 0.86)
            XCTAssertEqual(t.material2, "glass"); XCTAssertEqual(t.material3, "glass")
            XCTAssertGreaterThanOrEqual(ThemeStore.ratio(ThemeStore.rgb(t.fg)!, ThemeStore.rgb(t.paper)!), 4.5)
            XCTAssertGreaterThanOrEqual(ThemeStore.ratio(ThemeStore.rgb(t.muted)!, ThemeStore.rgb(t.paper2)!), 4.5)
            XCTAssertGreaterThanOrEqual(ThemeStore.ratio(ThemeStore.rgb(t.onScrim)!, ThemeStore.rgb("#000000")!), 4.5)
        }
    }

    func testTheBodyReadsInTheWebsSans() {                                     // D7
        for (mode, _) in [("light", 0), ("dark", 0)] {
            let s = store(fresh()); s.setMode(mode)
            XCTAssertEqual(s.current().fontRead, "system-ui"); XCTAssertEqual(s.current().fontUi, "system-ui")
        }
    }

    func testRemovingTheChosenThemeReturnsToTheDefault() {                     // D8
        let d = fresh()
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("defaults-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let s = ThemeStore(userDir: dir, defaults: d, systemScheme: { "dark" })
        let ember = ##"{"schema_version":3,"id":"ember","name":"Ember","colors":{"paper":"#FFFFFF","fg":"#111111","muted":"#444444","accent":"#0000CC","onAccent":"#FFFFFF"}}"##
        XCTAssertNoThrow(try s.installFromTexture(Data(ember.utf8)))
        s.setTheme("ember"); XCTAssertEqual(s.themeID, "ember")
        s.remove("ember")
        XCTAssertNil(d.string(forKey: "AutoradioThemeID"))
        XCTAssertEqual(s.themeID, "liquid-metal")
    }

    // Failure list, second round (review findings):
    //  D9  a Texture or a file installs a theme under a bundled id, replaces the bundled document and its origin, and remove deletes
    //      the default; the id "default" is taken by a theme and hides the Default entry;
    //  D10 a person who taps Chrome Pastel and then Dark is stuck light with no way back to the pair, or sees a half-dark screen;
    //  D11 the Default entry is missing, is not first, is not the one in use with no choice, or does not clear a choice.

    func testNothingReplacesABundledTheme() {                                  // D9
        let d = fresh()
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("defaults-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let s = ThemeStore(userDir: dir, defaults: d, systemScheme: { "light" })
        let paper = s.current().paper
        let clash = ##"{"schema_version":3,"id":"chrome-pastel","name":"Fake","colors":{"paper":"#FFFFFF","fg":"#111111","muted":"#444444","accent":"#0000CC","onAccent":"#FFFFFF"}}"##
        XCTAssertThrowsError(try s.installFromTexture(Data(clash.utf8)))
        XCTAssertEqual(s.current().paper, paper); XCTAssertEqual(s.cards().first { $0.id == "chrome-pastel" }?.name, "Chrome Pastel")
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.appendingPathComponent("texture-chrome-pastel.json").path), "nothing is written")
        try? Data(clash.utf8).write(to: dir.appendingPathComponent("chrome-pastel.json"))
        s.reload()
        XCTAssertTrue(s.problems.contains { $0.contains("bundled") })
        s.remove("chrome-pastel")
        XCTAssertEqual(s.themeID, "chrome-pastel"); XCTAssertEqual(s.current().paper, paper)
        let reserved = ##"{"schema_version":3,"id":"default","name":"Mine","colors":{"paper":"#FFFFFF","fg":"#111111","muted":"#444444","accent":"#0000CC","onAccent":"#FFFFFF"}}"##
        XCTAssertThrowsError(try s.installFromTexture(Data(reserved.utf8)))
    }

    func testAChosenSingleSchemeThemeLocksTheModeHonestlyAndTheDefaultGivesItBack() {   // D10
        let d = fresh()
        let s = store(d, system: "dark"); s.setMode("dark")
        s.setTheme("chrome-pastel")
        XCTAssertEqual(s.lockedScheme, "light")
        XCTAssertEqual(s.schemeMode, "light", "the mode control shows the one scheme the theme has")
        s.setMode("dark")
        XCTAssertEqual(s.schemeMode, "light", "and refuses the other"); XCTAssertEqual(s.effectiveScheme(), "light")
        XCTAssertEqual(s.current().scheme, "light"); XCTAssertEqual(s.current().id, "chrome-pastel")
        s.setTheme("depth")                                                       // a two-scheme theme gives the person's mode back
        XCTAssertNil(s.lockedScheme); XCTAssertEqual(s.schemeMode, "dark"); XCTAssertEqual(s.current().scheme, "dark")
        s.setTheme("chrome-pastel"); s.setTheme("default")                         // and so does the Default entry
        XCTAssertNil(s.lockedScheme); XCTAssertEqual(s.schemeMode, "dark")
        XCTAssertEqual(s.themeID, "liquid-metal"); XCTAssertEqual(s.current().scheme, "dark")
    }

    func testTheDefaultCardPreviewsTheDefaultNotTheChosenTheme() {              // D12
        let light = store(fresh(), system: "light"); light.setTheme("lantern")
        let a = light.cards().first
        XCTAssertEqual(a?.id, "default")
        XCTAssertEqual(a?.paper, "#F7FBFE", "Chrome Pastel, though Lantern is chosen")
        XCTAssertNotEqual(a?.paper, light.cards().first { $0.id == "lantern" }?.paper)
        let dark = store(fresh(), system: "dark"); dark.setTheme("depth")
        XCTAssertEqual(dark.cards().first?.paper, "#1C242E", "Liquid Metal in a dark scheme")
        let locked = store(fresh(), system: "dark"); locked.setTheme("chrome-pastel")   // light-locked, the phone asks dark
        XCTAssertEqual(locked.cards().first?.paper, "#1C242E", "the Default previews what selecting it would show")
    }

    func testTheDefaultEntryIsFirstClearsTheChoiceAndIsInUseOnlyWithoutOne() {   // D11
        let d = fresh()
        let s = store(d, system: "light")
        XCTAssertEqual(s.cards().first?.id, "default"); XCTAssertEqual(s.cards().first?.name, "Default")
        XCTAssertEqual(s.cards().filter { $0.current }.map(\.id), ["default"])
        s.setTheme("lantern")
        XCTAssertEqual(s.cards().filter { $0.current }.map(\.id), ["lantern"])
        XCTAssertEqual(d.string(forKey: "AutoradioThemeID"), "lantern")
        s.setTheme("default")
        XCTAssertNil(d.string(forKey: "AutoradioThemeID"))
        XCTAssertEqual(s.cards().filter { $0.current }.map(\.id), ["default"])
        XCTAssertEqual(s.themeID, "chrome-pastel")
        XCTAssertEqual(store(d, system: "light").themeID, "chrome-pastel", "and it survives a relaunch")
    }
}
