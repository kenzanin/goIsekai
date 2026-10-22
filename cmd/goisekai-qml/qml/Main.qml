import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "components"

// Window shell. Owns all filter state so the toolbar, sidebar and grid stay
// stateless and cannot reset each other.
ApplicationWindow {
    id: root

    property var allItems: []
    property var facets: ({
    })
    property var tags: ({
    })
    property string query: ""
    property string sortKey: "updated"
    property string statusFilter: "all"
    property string pluginFilter: ""
    property string tagFilter: ""
    property bool onlyDuplicates: false
    property int page: 1
    readonly property int pageSize: 24
    // Visible rows for the current filter set, rebuilt by rebuild().
    property var rows: []
    // Set by refresh(); onLastRefreshChanged swaps the data in. A plain
    // property var cannot be reassigned from Go, so the round trip goes
    // through QML's own XMLHttpRequest.
    property var lastRefresh: null

    function byTitle(a, b) {
        return a.title.localeCompare(b.title);
    }

    function cmpDate(a, b) {
        if (!a || !b)
            return (b ? 1 : 0) - (a ? 1 : 0);

        return a < b ? 1 : (a > b ? -1 : 0); // newest first
    }

    // Mirrors internal/httpserver/library_sort.go. Doing this client-side keeps
    // the host unchanged; 532 rows sort in a few ms.
    function sorted(rows) {
        const out = rows.slice();
        if (sortKey === "title")
            out.sort(byTitle);
        else if (sortKey === "read")
            out.sort((a, b) => {
            return b.read - a.read || byTitle(a, b);
        });
        else if (sortKey === "progress")
            out.sort((a, b) => {
            const pa = a.total > 0 ? a.read / a.total : 0;
            const pb = b.total > 0 ? b.read / b.total : 0;
            return pb - pa || byTitle(a, b);
        });
        else if (sortKey === "recent_read")
            out.sort((a, b) => {
            return cmpDate(a.lastReadAt, b.lastReadAt) || byTitle(a, b);
        });
        else if (sortKey === "duplicates")
            out.sort((a, b) => {
            return (a.dupGroup < 0 ? 999 : a.dupGroup) - (b.dupGroup < 0 ? 999 : b.dupGroup) || byTitle(a, b);
        });
        else
            out.sort((a, b) => {
            return cmpDate(a.updatedAt, b.updatedAt) || byTitle(a, b);
        });
        return out;
    }

    function matchesStatus(it) {
        if (statusFilter === "reading")
            return it.read > 0 && it.total > 0 && it.read < it.total;

        if (statusFilter === "unread")
            return it.read === 0;

        if (statusFilter === "done")
            return it.total > 0 && it.read >= it.total;

        if (statusFilter === "new")
            return it.hasNew;

        return true;
    }

    function hasTag(it) {
        if (tagFilter === "")
            return true;

        const list = tags[it.key] || [];
        const want = tagFilter.toLowerCase();
        for (let i = 0; i < list.length; ++i) {
            if (String(list[i]).toLowerCase() === want)
                return true;

        }
        return false;
    }

    function rebuild() {
        const q = query.trim().toLowerCase();
        let kept = allItems.filter((it) => {
            if (!matchesStatus(it))
                return false;

            if (pluginFilter !== "" && it.pluginId !== pluginFilter)
                return false;

            if (!hasTag(it))
                return false;

            if (onlyDuplicates && it.dupGroup < 0)
                return false;

            if (q !== "" && it.title.toLowerCase().indexOf(q) === -1)
                return false;

            return true;
        });
        rows = sorted(kept);
        // Only fall back to page 1 when the current page fell off the end;
        // resetting on every keystroke would fight the user.
        if ((page - 1) * pageSize >= rows.length)
            page = 1;

        grid.currentIndex = 0;
    }

    function openItem(it) {
        if (it && it.href)
            Qt.openUrlExternally(it.href);

    }

    function refresh() {
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function() {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return ;

            if (xhr.status !== 200) {
                statusText = "refresh failed — HTTP " + xhr.status;
                return ;
            }
            try {
                lastRefresh = JSON.parse(xhr.responseText);
            } catch (e) {
                statusText = "refresh failed — bad JSON";
            }
        };
        xhr.open("GET", serverBase + "/api/library");
        xhr.send();
    }

    visible: true
    width: 1280
    height: 860
    minimumWidth: 720
    minimumHeight: 480
    title: "goIsekai — Library"
    color: Theme.bg
    onLastRefreshChanged: {
        if (lastRefresh && lastRefresh.length !== undefined) {
            allItems = lastRefresh;
            statusText = allItems.length + " manga";
            rebuild();
        }
    }
    Component.onCompleted: {
        allItems = JSON.parse(libraryJson);
        facets = JSON.parse(facetsJson);
        tags = facets.tags || ({
        });
        rebuild();
    }

    Shortcut {
        sequence: "F5"
        onActivated: root.refresh()
    }

    Shortcut {
        sequences: ["Ctrl+F", "Ctrl+f"]
        onActivated: toolbar.focusSearch()
    }

    RowLayout {
        anchors.fill: parent
        spacing: 0

        LibrarySidebar {
            Layout.fillHeight: true
            Layout.preferredWidth: 240
            facets: root.facets
            pluginFilter: root.pluginFilter
            tagFilter: root.tagFilter
            onlyDuplicates: root.onlyDuplicates
            onPluginToggled: (id) => {
                root.pluginFilter = (root.pluginFilter === id) ? "" : id;
                root.rebuild();
            }
            onTagToggled: (tag) => {
                root.tagFilter = (root.tagFilter === tag) ? "" : tag;
                root.rebuild();
            }
            onDuplicatesToggled: {
                root.onlyDuplicates = !root.onlyDuplicates;
                root.rebuild();
            }
            onAnyFilterCleared: {
                root.pluginFilter = "";
                root.tagFilter = "";
                root.onlyDuplicates = false;
                root.rebuild();
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            Pagination {
                Layout.fillWidth: true
                Layout.margins: 8
                page: root.page
                pageCount: Math.max(1, Math.ceil(root.rows.length / root.pageSize))
                onRequested: (p) => {
                    root.page = p;
                    grid.forceActiveFocus();
                }
            }

            MangaGrid {
                id: grid

                Layout.fillWidth: true
                Layout.fillHeight: true
                rows: root.rows
                page: root.page
                pageSize: root.pageSize
                totalRows: root.allItems.length
                queryText: root.query
                onActivated: (it) => {
                    return root.openItem(it);
                }
            }

            Pagination {
                Layout.fillWidth: true
                Layout.margins: 8
                page: root.page
                pageCount: Math.max(1, Math.ceil(root.rows.length / root.pageSize))
                onRequested: (p) => {
                    root.page = p;
                    grid.forceActiveFocus();
                }
            }

        }

    }

    header: LibraryToolbar {
        id: toolbar

        query: root.query
        sortKey: root.sortKey
        statusFilter: root.statusFilter
        statusText: root.statusText
        resultCount: root.rows.length
        totalCount: root.allItems.length
        onQueryEdited: (v) => {
            root.query = v;
            root.rebuild();
        }
        onSortEdited: (v) => {
            root.sortKey = v;
            root.rebuild();
        }
        onStatusEdited: (v) => {
            root.statusFilter = v;
            root.rebuild();
        }
        onRefreshRequested: root.refresh()
        onSearchFocusReturned: grid.forceActiveFocus()
        onOpenRequested: root.openItem(root.rows[grid.currentIndex])
    }

}
