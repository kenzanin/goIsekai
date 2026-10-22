import QtQuick
import QtTest
import "../qml"
import "../qml/components"

// Covers the derived properties that decide what a card shows. These are pure
// functions of `item`, so they can be tested without a server, a window, or the
// Go side - which is exactly why they live in QML rather than in the model.
TestCase {
    id: tc

    name: "MangaCard"

    function makeCard(item) {
        const card = Qt.createComponent("../qml/components/MangaCard.qml")
        verify(card.status === Component.Ready, "MangaCard failed to compile: " + card.errorString())
        return card.createObject(tc, {
            "item": item
        })
    }

    function test_status_done_wins_over_new() {
        // A finished manga that also has a new chapter: "finished" is the more
        // useful badge, and matching the filter matters more than the dot.
        const c = makeCard({
            "read": 12,
            "total": 12,
            "hasNew": true
        })
        compare(c.statusKey, "done")
        c.destroy()
    }

    function test_status_new_when_unfinished() {
        const c = makeCard({
            "read": 3,
            "total": 12,
            "hasNew": true
        })
        compare(c.statusKey, "new")
        c.destroy()
    }

    function test_status_reading() {
        const c = makeCard({
            "read": 3,
            "total": 12,
            "hasNew": false
        })
        compare(c.statusKey, "reading")
        c.destroy()
    }

    function test_status_unread_when_no_progress() {
        const c = makeCard({
            "read": 0,
            "total": 12,
            "hasNew": false
        })
        compare(c.statusKey, "unread")
        c.destroy()
    }

    function test_status_unknown_total_is_not_done() {
        // total == 0 means the chapter count is unknown, not finished. Getting
        // this backwards would paint the whole library green.
        const c = makeCard({
            "read": 0,
            "total": 0,
            "hasNew": false
        })
        compare(c.statusKey, "unread")
        c.destroy()
    }

    function test_status_empty_without_item() {
        const c = makeCard(null)
        compare(c.statusKey, "")
        c.destroy()
    }

    function test_status_style_covers_every_key() {
        // A missing entry would render an undefined colour and a blank badge.
        const c = makeCard(null)
        const keys = ["done", "new", "reading", "unread"]
        for (let i = 0; i < keys.length; ++i) {
            verify(c.statusStyle[keys[i]] !== undefined, "statusStyle has no entry for " + keys[i])
            verify(c.statusStyle[keys[i]].label !== "", "label missing for " + keys[i])
        }
        c.destroy()
    }

    function test_plugin_name_defaults_empty() {
        const c = makeCard({
            "read": 0,
            "total": 1,
            "hasNew": false
        })
        // An unset name must hide the overlay rather than show "undefined".
        compare(c.pluginName, "")
        c.destroy()
    }
}
