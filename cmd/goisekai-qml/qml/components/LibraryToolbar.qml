import ".."
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// Search, sort, status chips, refresh. Holds no state of its own: every control
// reports the value it wants and Main.qml decides. That is what stops one
// control from silently clearing another.
ToolBar {
    id: bar

    property string query: ""
    property string sortKey: "updated"
    property string statusFilter: "all"
    property string statusText: ""
    property int resultCount: 0
    property int totalCount: 0
    // Label/value pairs for the sort dropdown, mirroring librarySorts.
    readonly property var sortOptions: [{
        "key": "updated",
        "label": "Recently updated"
    }, {
        "key": "title",
        "label": "Title A–Z"
    }, {
        "key": "read",
        "label": "Most chapters read"
    }, {
        "key": "progress",
        "label": "Progress"
    }, {
        "key": "recent_read",
        "label": "Recently read"
    }, {
        "key": "duplicates",
        "label": "Duplicates first"
    }]
    readonly property var statusOptions: [{
        "key": "all",
        "label": "All"
    }, {
        "key": "reading",
        "label": "In progress"
    }, {
        "key": "unread",
        "label": "Unread"
    }, {
        "key": "done",
        "label": "Finished"
    }, {
        "key": "new",
        "label": "New"
    }]

    signal queryEdited(string value)
    signal sortEdited(string value)
    signal statusEdited(string value)
    signal refreshRequested()
    signal searchFocusReturned()
    signal openRequested()

    function focusSearch() {
        search.forceActiveFocus();
        search.selectAll();
    }

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: Theme.gap
        anchors.rightMargin: Theme.gap
        spacing: 12

        Label {
            text: "goIsekai"
            color: Theme.text
            font.pixelSize: 18
            font.bold: true
        }

        TextField {
            id: search

            Layout.preferredWidth: 260
            placeholderText: "Search title…  (Ctrl+F)"
            color: Theme.text
            placeholderTextColor: Theme.textFaint
            selectByMouse: true
            text: bar.query
            onTextChanged: {
                if (text !== bar.query)
                    bar.queryEdited(text);

            }
            Keys.onDownPressed: bar.searchFocusReturned()
            Keys.onReturnPressed: bar.openRequested()
            Keys.onEscapePressed: {
                text = "";
                bar.searchFocusReturned();
            }

            background: Rectangle {
                color: Theme.field
                radius: 6
                border.width: search.activeFocus ? 1 : 0
                border.color: Theme.accent
            }

        }

        ComboBox {
            id: sortPicker

            Layout.preferredWidth: 170
            model: bar.sortOptions.map((o) => {
                return o.label;
            })
            currentIndex: Math.max(0, bar.sortOptions.findIndex((o) => {
                return o.key === bar.sortKey;
            }))
            onActivated: (i) => {
                return bar.sortEdited(bar.sortOptions[i].key);
            }
        }

        Row {
            spacing: 6

            Repeater {
                model: bar.statusOptions

                delegate: Button {
                    required property var modelData
                    readonly property bool on: modelData.key === bar.statusFilter

                    text: modelData.label
                    padding: 8
                    font.pixelSize: 12
                    onClicked: bar.statusEdited(modelData.key)

                    background: Rectangle {
                        radius: 6
                        color: parent.on ? "#3a2a10" : Theme.field
                        border.width: 1
                        border.color: parent.on ? Theme.accent : "transparent"
                    }

                    contentItem: Label {
                        text: parent.text
                        color: parent.on ? Theme.accent : Theme.textDim
                        font.pixelSize: 12
                        horizontalAlignment: Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                    }

                }

            }

        }

        Item {
            Layout.fillWidth: true
        }

        Label {
            // Show the filtered count only when a filter narrowed the list.
            text: resultCount === totalCount ? totalCount + " manga" : resultCount + " of " + totalCount + " manga"
            color: Theme.textDim
            font.pixelSize: 13
        }

        Button {
            text: "Refresh (F5)"
            font.pixelSize: 12
            onClicked: bar.refreshRequested()
        }

    }

    background: Rectangle {
        color: Theme.panel
    }

}
