import QtQuick
import QtTest
import "../qml"
import "../qml/components"

// The page-number window is the one piece of real arithmetic in the UI: it has
// to stay navigable for a 20+ page library without rendering 20 buttons.
TestCase {
    id: tc

    name: "Pagination"

    function makePager(page, pageCount) {
        const c = Qt.createComponent("../qml/components/Pagination.qml")
        verify(c.status === Component.Ready, "Pagination failed to compile: " + c.errorString())
        return c.createObject(tc, {
            "page": page,
            "pageCount": pageCount
        })
    }

    function test_short_library_lists_every_page() {
        const p = makePager(1, 5)
        compare(p.pages.length, 5)
        compare(p.pages.join(","), "1,2,3,4,5")
        p.destroy()
    }

    function test_long_library_is_windowed() {
        // First, last, and a +/-1 window around the current page.
        const p = makePager(10, 22)
        compare(p.pages.join(","), "1,-1,9,10,11,-1,22")
        p.destroy()
    }

    function test_window_at_first_page_has_no_leading_ellipsis() {
        const p = makePager(1, 22)
        compare(p.pages.join(","), "1,2,-1,22")
        p.destroy()
    }

    function test_window_at_last_page_has_no_trailing_ellipsis() {
        const p = makePager(22, 22)
        compare(p.pages.join(","), "1,-1,21,22")
        p.destroy()
    }

    function test_always_shows_first_and_last() {
        const p = makePager(5, 12)
        compare(p.pages[0], 1)
        compare(p.pages[p.pages.length - 1], 12)
        p.destroy()
    }

    function test_current_page_is_inside_the_window() {
        const p = makePager(7, 30)
        verify(p.pages.indexOf(7) !== -1, "current page missing from " + p.pages.join(","))
        p.destroy()
    }

    function test_single_page() {
        const p = makePager(1, 1)
        compare(p.pages.join(","), "1")
        p.destroy()
    }
}
