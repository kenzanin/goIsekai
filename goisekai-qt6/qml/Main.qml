import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Goisekai

// Window shell. Owns all filter state so the toolbar, sidebar and grid stay
// stateless and cannot reset each other.
ApplicationWindow {
    id: root

    property var allItems: []
    property var facets: ({})
    property var pluginNames: ({})
    property string query: ""
    property string sortKey: "updated"
    property string statusFilter: "all"
    property string pluginFilter: ""
    property bool onlyDuplicates: false
    property int page: 1
    readonly property int pageSize: 25
    property string currentTab: "library"
    property var rows: []
    property string statusText: "loading…"

    visible: true
    width: 1280
    height: 860
    minimumWidth: 720
    minimumHeight: 480
    title: "goIsekai — Library"
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

    function rebuild() {
        const q = query.trim().toLowerCase();
        const kept = allItems.filter((it) => {
            if (pluginFilter !== "" && it.pluginId !== pluginFilter)
                return false;
            if (onlyDuplicates && it.dupGroup < 0)
                return false;
            if (statusFilter === "reading")
                return it.read > 0 && it.total > 0 && it.read < it.total;
            if (statusFilter === "unread")
                return it.read === 0;
            if (statusFilter === "done")
                return it.total > 0 && it.read >= it.total;
            if (statusFilter === "new" && !it.hasNew)
                return false;
            if (q !== "" && it.title.toLowerCase().indexOf(q) === -1)
                return false;
            return true;
        });
        kept.sort((a, b) => {
            if (sortKey === "title")
                return a.title.localeCompare(b.title);
            if (sortKey === "read")
                return b.read - a.read || a.title.localeCompare(b.title);
            if (sortKey === "progress") {
                const pa = a.total > 0 ? a.read / a.total : 0;
                const pb = b.total > 0 ? b.read / b.total : 0;
                return pb - pa || a.title.localeCompare(b.title);
            }
            if (sortKey === "recent_read")
                return (b.lastReadAt || "").localeCompare(a.lastReadAt || "")
                       || a.title.localeCompare(b.title);
            if (sortKey === "duplicates")
                return (a.dupGroup < 0 ? 999 : a.dupGroup)
                       - (b.dupGroup < 0 ? 999 : b.dupGroup)
                       || a.title.localeCompare(b.title);
            return (b.updatedAt || "").localeCompare(a.updatedAt || "")
                   || a.title.localeCompare(b.title);
        });
        rows = kept;
        if ((page - 1) * pageSize >= rows.length && page > 1)
            page = 1;
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

    // Placeholder for every tab except the library.
    Rectangle {
        anchors.fill: parent
        visible: root.currentTab !== "library"
        color: Theme.bg

        Label {
            anchors.centerIn: parent
            text: root.currentTab.charAt(0).toUpperCase() + root.currentTab.slice(1)
            color: Theme.textDim
            font.pixelSize: 28
        }
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0
        visible: root.currentTab === "library"

        // Top bar: brand doubles as the library tab.
        Rectangle {
            Layout.fillWidth: true
            implicitHeight: 48
            color: Theme.panel

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: Theme.gap
                anchors.rightMargin: Theme.gap
                spacing: 4

                Repeater {
                    model: [
                        { "key": "library", "label": "goIsekai" },
                        { "key": "search", "label": "Search" },
                        { "key": "updates", "label": "Updates" },
                        { "key": "history", "label": "History" },
                        { "key": "plugins", "label": "Plugins" },
                        { "key": "settings", "label": "Settings" },
                        { "key": "logs", "label": "Logs" },
                        { "key": "about", "label": "About" }
                    ]

                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool on: modelData.key === root.currentTab
                        readonly property bool brand: modelData.key === "library"

                        Layout.preferredHeight: 34
                        Layout.alignment: Qt.AlignVCenter
                        implicitWidth: tabLabel.implicitWidth + (brand ? 28 : 24)
                        radius: 6
                        color: on ? "#3a2a10" : "transparent"

                        Label {
                            id: tabLabel
                            anchors.centerIn: parent
                            text: modelData.label
                            color: parent.on ? Theme.accent
                                             : (parent.brand ? Theme.text : Theme.textDim)
                            font.pixelSize: 13
                            font.bold: parent.brand
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.currentTab = modelData.key
                        }
                    }
                }

                Item { Layout.fillWidth: true }

                Label {
                    text: root.statusText
                    color: Theme.textFaint
                    font.pixelSize: 12
                }
            }
        }

        // Toolbar: search, sort, status, refresh.
        Rectangle {
            Layout.fillWidth: true
            implicitHeight: 52
            color: Theme.panel

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: Theme.gap
                anchors.rightMargin: Theme.gap
                spacing: 12

                TextField {
                    id: search
                    Layout.preferredWidth: 260
                    placeholderText: "Search title… (Ctrl+F)"
                    color: Theme.text
                    placeholderTextColor: Theme.textFaint
                    selectByMouse: true
                    onTextChanged: {
                        if (text !== root.query) {
                            root.query = text;
                            root.rebuild();
                        }
                    }
                }

                ComboBox {
                    Layout.preferredWidth: 180
                    model: ["Recently updated", "Title A–Z", "Most chapters read",
                            "Progress", "Recently read", "Duplicates first"]
                    onActivated: (i) => {
                        root.sortKey = ["updated", "title", "read", "progress",
                                        "recent_read", "duplicates"][i];
                        root.rebuild();
                    }
                }

                ComboBox {
                    Layout.preferredWidth: 140
                    model: ["All", "In progress", "Unread", "Finished", "New"]
                    onActivated: (i) => {
                        root.statusFilter = ["all", "reading", "unread", "done", "new"][i];
                        root.rebuild();
                    }
                }

                Item { Layout.fillWidth: true }

                Button {
                    text: "Refresh"
                    onClicked: root.loadLibrary()
                }
            }
        }

        // Grid.
        GridView {
            id: grid
            Layout.fillWidth: true
            Layout.fillHeight: true
            Layout.margins: Theme.gap
            clip: true
            cellWidth: 180
            cellHeight: 320
            model: root.rows.slice((root.page - 1) * root.pageSize,
                                   root.page * root.pageSize)

            delegate: Item {
                required property var modelData
                width: grid.cellWidth
                height: grid.cellHeight

                MangaCard {
                    anchors.fill: parent
                    anchors.margins: 6
                    item: modelData
                    pluginName: root.pluginNames[modelData.pluginId] || ""
                    onActivated: (it) => root.openItem(it)
                }
            }

            Label {
                anchors.centerIn: parent
                visible: root.rows.length === 0
                text: root.statusText
                color: Theme.textDim
                font.pixelSize: 14
            }
        }

        // Pagination.
        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 8
            spacing: 8

            Item { Layout.fillWidth: true }

            Button {
                text: "‹"
                enabled: root.page > 1
                onClicked: root.page--
            }

            Label {
                text: root.page + " / " + Math.max(1, Math.ceil(root.rows.length / root.pageSize))
                color: Theme.textDim
                font.pixelSize: 13
            }

            Button {
                text: "›"
                enabled: root.page < Math.ceil(root.rows.length / root.pageSize)
                onClicked: root.page++
            }

            Item { Layout.fillWidth: true }
        }
    }
}
