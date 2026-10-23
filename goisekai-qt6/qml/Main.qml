import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Goisekai
import ".."

// Window shell. Owns all filter state so the toolbar, sidebar and grid stay
// stateless and cannot reset each other. The nav bar lives in `header`, so it
// stays visible on every tab; only the content area swaps.
ApplicationWindow {
    id: root

    property var allItems: []
    property var facets: ({})
    property var pluginNames: ({})
    property var tags: ({})
    property string query: ""
    property string sortKey: "updated"
    property string statusFilter: "all"
    property string pluginFilter: ""
    property string tagFilter: ""
    property bool onlyDuplicates: false
    property int page: 1
    readonly property int pageSize: 25
    property string currentTab: "library"
    property var rows: []
    property string statusText: "loading…"

    readonly property var navTabs: [
        { "key": "library", "label": "goIsekai" },
        { "key": "search", "label": "Search" },
        { "key": "updates", "label": "Updates" },
        { "key": "history", "label": "History" },
        { "key": "plugins", "label": "Plugins" },
        { "key": "settings", "label": "Settings" },
        { "key": "logs", "label": "Logs" },
        { "key": "about", "label": "About" }
    ]
    readonly property var sortKeys: ["updated", "title", "read", "progress",
                                     "recent_read", "duplicates"]
    readonly property var statusKeys: ["all", "reading", "unread", "done", "new"]

    visible: true
    width: 1280
    height: 860
    minimumWidth: 720
    minimumHeight: 480
    title: "goIsekai"
    color: Theme.bg

    // GET /api/library rows -> the shape the cards read.
    function fromAPI(list) {
        return list.map((r) => ({
            "title": r.title,
            "cover": r.cover_url
                     ? serverBase + "/image?pluginID=" + encodeURIComponent(r.plugin_id)
                       + "&url=" + encodeURIComponent(r.cover_url)
                     : "",
            "read": r.read_chapters,
            "total": r.total_chapters,
            "hasNew": r.has_new,
            "updatedAt": r.updated_at,
            "lastReadAt": r.last_read_at,
            "dupGroup": r.dup_group,
            "href": serverBase + "/view/manga/" + encodeURIComponent(r.plugin_id)
                    + "/" + encodeURIComponent(r.source_manga_id),
            "key": r.plugin_id + ":" + r.source_manga_id,
            "pluginId": r.plugin_id
        }));
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

    function cmpDate(a, b) {
        if (!a || !b)
            return (b ? 1 : 0) - (a ? 1 : 0);
        return a < b ? 1 : (a > b ? -1 : 0);
    }

    function byTitle(a, b) {
        return a.title.localeCompare(b.title);
    }

    // Mirrors internal/httpserver/library_sort.go.
    function sorted(list) {
        const out = list.slice();
        if (sortKey === "title")
            out.sort(byTitle);
        else if (sortKey === "read")
            out.sort((a, b) => b.read - a.read || byTitle(a, b));
        else if (sortKey === "progress")
            out.sort((a, b) => {
                const pa = a.total > 0 ? a.read / a.total : 0;
                const pb = b.total > 0 ? b.read / b.total : 0;
                return pb - pa || byTitle(a, b);
            });
        else if (sortKey === "recent_read")
            out.sort((a, b) => cmpDate(a.lastReadAt, b.lastReadAt) || byTitle(a, b));
        else if (sortKey === "duplicates")
            out.sort((a, b) => (a.dupGroup < 0 ? 999 : a.dupGroup)
                               - (b.dupGroup < 0 ? 999 : b.dupGroup)
                               || byTitle(a, b));
        else
            out.sort((a, b) => cmpDate(a.updatedAt, b.updatedAt) || byTitle(a, b));
        return out;
    }

    function rebuild() {
        const q = query.trim().toLowerCase();
        const kept = allItems.filter((it) => {
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
        // resetting per keystroke would fight the user.
        if ((page - 1) * pageSize >= rows.length && page > 1)
            page = 1;
        statusText = rows.length + " manga";
    }

    function openItem(it) {
        if (it && it.href)
            Qt.openUrlExternally(it.href);
    }

    function loadLibrary() {
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function () {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return;
            if (xhr.status !== 200) {
                statusText = "server unreachable — HTTP " + xhr.status;
                return;
            }
            try {
                allItems = root.fromAPI(JSON.parse(xhr.responseText));
                statusText = allItems.length + " manga";
                rebuild();
            } catch (e) {
                statusText = "bad JSON from /api/library";
            }
        };
        xhr.open("GET", serverBase + "/api/library");
        xhr.send();
    }

    function loadFacets() {
        const xhr = new XMLHttpRequest();
        xhr.onreadystatechange = function () {
            if (xhr.readyState !== XMLHttpRequest.DONE || xhr.status !== 200)
                return;
            try {
                facets = JSON.parse(xhr.responseText);
                tags = facets.tags || ({});
                const names = ({});
                const list = facets.plugins || [];
                for (let i = 0; i < list.length; ++i)
                    names[list[i].plugin_id] = list[i].name;
                pluginNames = names;
            } catch (e) {
                // Facets are optional; the grid still renders.
            }
        };
        xhr.open("GET", serverBase + "/api/library/facets");
        xhr.send();
    }

    Component.onCompleted: {
        loadLibrary();
        loadFacets();
    }

    Shortcut {
        sequence: "F5"
        onActivated: root.loadLibrary()
    }

    Shortcut {
        sequences: ["Ctrl+F", "Ctrl+f"]
        onActivated: {
            search.forceActiveFocus();
            search.selectAll();
        }
    }

    // Nav bar: always visible, on every tab (mirrors partials/nav.lua).
    header: Rectangle {
        implicitHeight: 52
        color: Theme.panel

        Rectangle {
            anchors.bottom: parent.bottom
            anchors.left: parent.left
            anchors.right: parent.right
            height: 1
            color: Theme.line
        }

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: Theme.gap
            anchors.rightMargin: Theme.gap
            spacing: 4

            Repeater {
                model: root.navTabs

                delegate: Rectangle {
                    required property var modelData
                    readonly property bool on: modelData.key === root.currentTab
                    readonly property bool brand: modelData.key === "library"

                    Layout.preferredHeight: 36
                    Layout.alignment: Qt.AlignVCenter
                    implicitWidth: navLabel.implicitWidth + (brand ? 28 : 24)
                    radius: 6
                    color: on ? Theme.accentWash : (navMouse.containsMouse ? Theme.field : "transparent")

                    Label {
                        id: navLabel
                        anchors.centerIn: parent
                        text: modelData.label
                        color: parent.on ? Theme.accentText
                                         : (parent.brand ? Theme.text : Theme.textDim)
                        font.pixelSize: Theme.fsBody
                        font.bold: parent.brand
                    }

                    // Active underline, like the web nav's border-b-2.
                    Rectangle {
                        visible: parent.on
                        anchors.bottom: parent.bottom
                        anchors.left: parent.left
                        anchors.right: parent.right
                        height: 2
                        color: Theme.accentSoft
                    }

                    MouseArea {
                        id: navMouse
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.currentTab = modelData.key
                    }
                }
            }

            Item { Layout.fillWidth: true }

            Label {
                Layout.alignment: Qt.AlignVCenter
                text: root.statusText
                color: Theme.textFaint
                font.pixelSize: Theme.fsSmall
            }
        }
    }

    // ---- Library page ------------------------------------------------------
    RowLayout {
        anchors.fill: parent
        spacing: 0
        visible: root.currentTab === "library"

        // Sidebar: stats, plugin counts, tag chips (library_sidebar.lua).
        Rectangle {
            Layout.fillHeight: true
            Layout.preferredWidth: 264
            color: Theme.panel

            Rectangle {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1
                color: Theme.line
            }

            Flickable {
                anchors.fill: parent
                anchors.margins: 12
                clip: true
                contentHeight: sideCol.implicitHeight
                boundsBehavior: Flickable.StopAtBounds

                ColumnLayout {
                    id: sideCol
                    width: parent.width
                    spacing: 10

                    // --- Stats cards
                    Rectangle {
                        Layout.fillWidth: true
                        implicitHeight: statCol.implicitHeight + 20
                        color: Theme.card
                        radius: Theme.radius
                        border.width: 1
                        border.color: Theme.line

                        ColumnLayout {
                            id: statCol
                            anchors.fill: parent
                            anchors.margins: 10
                            spacing: 2

                            Label {
                                text: root.facets.overview
                                      ? String(root.facets.overview.total_titles) : "0"
                                color: Theme.accentText
                                font.pixelSize: 26
                                font.bold: true
                            }
                            Label {
                                text: "Titles"
                                color: Theme.textDim
                                font.pixelSize: Theme.fsSmall
                            }
                            Label {
                                text: root.facets.overview
                                      ? (root.facets.overview.fully_read + " finished · "
                                         + root.facets.overview.started_reading + " reading")
                                      : ""
                                color: Theme.textFaint
                                font.pixelSize: Theme.fsSmall
                                Layout.fillWidth: true
                                wrapMode: Text.WordWrap
                            }
                        }
                    }

                    // --- Status filter chips
                    Label {
                        text: "Status"
                        color: Theme.textFaint
                        font.pixelSize: Theme.fsSmall
                        Layout.topMargin: 6
                    }

                    Repeater {
                        model: [
                            { "key": "all", "label": "All" },
                            { "key": "reading", "label": "In progress" },
                            { "key": "unread", "label": "Unread" },
                            { "key": "done", "label": "Finished" },
                            { "key": "new", "label": "New" }
                        ]

                        delegate: Rectangle {
                            required property var modelData
                            readonly property bool on: modelData.key === root.statusFilter

                            Layout.fillWidth: true
                            implicitHeight: 32
                            radius: 6
                            color: on ? Theme.accentWash : (chipMouse.containsMouse ? Theme.card : "transparent")
                            border.width: 1
                            border.color: on ? Theme.accent : Theme.line

                            Label {
                                anchors.left: parent.left
                                anchors.leftMargin: 10
                                anchors.verticalCenter: parent.verticalCenter
                                text: modelData.label
                                color: parent.on ? Theme.accentText : Theme.textDim
                                font.pixelSize: Theme.fsSmall
                            }

                            MouseArea {
                                id: chipMouse
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    root.statusFilter = modelData.key;
                                    root.rebuild();
                                }
                            }
                        }
                    }

                    // --- Duplicates toggle
                    Rectangle {
                        Layout.fillWidth: true
                        Layout.topMargin: 6
                        implicitHeight: 32
                        radius: 6
                        color: root.onlyDuplicates ? Theme.accentWash
                                                   : (dupMouse.containsMouse ? Theme.card : "transparent")
                        border.width: 1
                        border.color: root.onlyDuplicates ? Theme.accent : Theme.line

                        Label {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.verticalCenter: parent.verticalCenter
                            text: "Duplicates only"
                                     + (root.facets.duplicate_count
                                        ? " (" + root.facets.duplicate_count + ")" : "")
                            color: root.onlyDuplicates ? Theme.accentText : Theme.textDim
                            font.pixelSize: Theme.fsSmall
                        }

                        MouseArea {
                            id: dupMouse
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                root.onlyDuplicates = !root.onlyDuplicates;
                                root.rebuild();
                            }
                        }
                    }

                    // --- Plugins
                    Label {
                        visible: pluginRepeater.count > 0
                        text: "Plugins"
                        color: Theme.textFaint
                        font.pixelSize: Theme.fsSmall
                        Layout.topMargin: 6
                    }

                    Repeater {
                        id: pluginRepeater
                        model: root.facets.plugins || []

                        delegate: Rectangle {
                            required property var modelData
                            readonly property bool on: modelData.plugin_id === root.pluginFilter

                            Layout.fillWidth: true
                            implicitHeight: 32
                            radius: 6
                            color: on ? Theme.accentWash : (plugMouse.containsMouse ? Theme.card : "transparent")
                            border.width: 1
                            border.color: on ? Theme.accent : Theme.line

                            RowLayout {
                                anchors.fill: parent
                                anchors.leftMargin: 10
                                anchors.rightMargin: 10
                                spacing: 6

                                Label {
                                    Layout.fillWidth: true
                                    text: modelData.name || modelData.plugin_id
                                    color: parent.parent.on ? Theme.accentText : Theme.textDim
                                    font.pixelSize: Theme.fsSmall + 3
                                    elide: Text.ElideRight
                                }
                                // Plugin logo (icon field from facets.plugins)
                                Rectangle {
                                    visible: modelData.icon !== ""
                                    width: 24
                                    height: 24
                                    radius: 4
                                    color: Theme.field
                                    Image {
                                        anchors.centerIn: parent
                                        source: serverBase + modelData.icon
                                        sourceSize.width: 16
                                        sourceSize.height: 16
                                        asynchronous: true
                                        cache: true
                                    }
                                }
                                Label {
                                    text: String(modelData.count)
                                    color: Theme.textFaint
                                    font.pixelSize: Theme.fsSmall + 3
                                }
                            }

                            MouseArea {
                                id: plugMouse
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    root.pluginFilter = (root.pluginFilter === modelData.plugin_id)
                                                        ? "" : modelData.plugin_id;
                                    root.rebuild();
                                }
                            }
                        }
                    }

                    // --- Cats (tags)
                    Label {
                        visible: catRepeater.count > 0
                        text: "Tags"
                        color: Theme.textFaint
                        font.pixelSize: Theme.fsSmall
                        Layout.topMargin: 6
                    }

                    Repeater {
                        id: catRepeater
                        model: root.facets.categories || []

                        delegate: Rectangle {
                            required property var modelData
                            readonly property bool on: modelData.name === root.tagFilter

                            Layout.fillWidth: true
                            implicitHeight: 32
                            radius: 6
                            color: on ? Theme.accentWash : (catMouse.containsMouse ? Theme.card : "transparent")
                            border.width: 1
                            border.color: on ? Theme.accent : Theme.line

                            RowLayout {
                                anchors.fill: parent
                                anchors.leftMargin: 10
                                anchors.rightMargin: 10
                                spacing: 6

                                Label {
                                    Layout.fillWidth: true
                                    text: modelData.name
                                    color: parent.parent.on ? Theme.accentText : Theme.textDim
                                    font.pixelSize: Theme.fsSmall + 3
                                    elide: Text.ElideRight
                                }
                                Label {
                                    text: String(modelData.count)
                                    color: Theme.textFaint
                                    font.pixelSize: Theme.fsSmall + 3
                                }
                            }

                            MouseArea {
                                id: catMouse
                                anchors.fill: parent
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    root.tagFilter = (root.tagFilter === modelData.name)
                                                     ? "" : modelData.name;
                                    root.rebuild();
                                }
                            }
                        }
                    }

                    Item { Layout.fillHeight: true }
                }
            }
        }

        // Content column: toolbar, grid, pagination.
        ColumnLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            spacing: 0

            // Toolbar (search / sort / view).
            Rectangle {
                Layout.fillWidth: true
                implicitHeight: 60
                color: Theme.bg

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: Theme.gap
                    anchors.rightMargin: Theme.gap
                    spacing: 10

                    Label {
                        text: "Library"
                        color: Theme.text
                        font.pixelSize: Theme.fsHeading
                        font.bold: true
                    }

                    TextField {
                        id: search
                        Layout.fillWidth: true
                        Layout.maximumWidth: 380
                        placeholderText: "Search library… (Ctrl+F)"
                        color: Theme.text
                        placeholderTextColor: Theme.textFaint
                        font.pixelSize: Theme.fsBody
                        selectByMouse: true
                        background: Rectangle {
                            color: Theme.field
                            radius: 6
                            border.width: search.activeFocus ? 1 : 1
                            border.color: search.activeFocus ? Theme.accent : Theme.lineStrong
                        }
                        onTextChanged: {
                            if (text !== root.query) {
                                root.query = text;
                                root.rebuild();
                            }
                        }
                        Keys.onReturnPressed: {
                            if (root.rows.length > 0)
                                root.openItem(root.rows[0]);
                        }
                        Keys.onDownPressed: grid.forceActiveFocus()
                    }

                    ComboBox {
                        Layout.preferredWidth: 200
                        font.pixelSize: Theme.fsSmall
                        model: ["Recently updated", "Title A–Z", "Most chapters read",
                                "Progress", "Recently read", "Duplicates first"]
                        currentIndex: Math.max(0, root.sortKeys.indexOf(root.sortKey))
                        onActivated: (i) => {
                            root.sortKey = root.sortKeys[i];
                            root.rebuild();
                        }
                    }

                    Item { Layout.fillWidth: true }

                    Button {
                        text: "Refresh"
                        font.pixelSize: Theme.fsSmall
                        onClicked: root.loadLibrary()
                    }
                }
            }

            // Grid.
            Flow {
                id: grid
                Layout.fillWidth: true
                Layout.fillHeight: true
                Layout.leftMargin: Theme.gap
                Layout.rightMargin: Theme.gap
                wrap: true
                spacing: 6
                model: root.rows.slice((root.page - 1) * root.pageSize,
                                       root.page * root.pageSize)

                delegate: MangaCard {
                    item: modelData
                    pluginName: root.pluginNames[modelData.pluginId] || ""
                    onActivated: (it) => root.openItem(it)
                }

                Label {
                    anchors.centerIn: parent
                    visible: root.rows.length === 0
                    text: root.query !== ""
                          ? "No manga matches “" + root.query + "” with these filters"
                          : root.statusText
                    color: Theme.textDim
                    font.pixelSize: Theme.fsBody
                }
            }

            // Pagination.
            RowLayout {
                Layout.fillWidth: true
                Layout.margins: 12
                spacing: 8

                Item { Layout.fillWidth: true }

                Button {
                    text: "‹ Prev"
                    font.pixelSize: Theme.fsSmall
                    enabled: root.page > 1
                    onClicked: root.page--
                }

                Label {
                    text: root.page + " / "
                          + Math.max(1, Math.ceil(root.rows.length / root.pageSize))
                    color: Theme.textDim
                    font.pixelSize: Theme.fsSmall
                }

                Button {
                    text: "Next ›"
                    font.pixelSize: Theme.fsSmall
                    enabled: root.page * root.pageSize < root.rows.length
                    onClicked: root.page++
                }

                Item { Layout.fillWidth: true }
            }
        }
    }

    // ---- Placeholder for every tab except the library ----------------------
    Rectangle {
        anchors.fill: parent
        visible: root.currentTab !== "library"
        color: Theme.bg

        ColumnLayout {
            anchors.centerIn: parent
            spacing: 8

            Label {
                Layout.alignment: Qt.AlignHCenter
                text: root.currentTab.charAt(0).toUpperCase() + root.currentTab.slice(1)
                color: Theme.text
                font.pixelSize: 30
                font.bold: true
            }
            Label {
                Layout.alignment: Qt.AlignHCenter
                text: "Not built yet — open it in the web UI."
                color: Theme.textDim
                font.pixelSize: Theme.fsBody
            }
        }
    }
}
