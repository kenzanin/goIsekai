import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One library card. Emits the item; Main.qml decides what a click does.
Rectangle {
    id: card

    property var item: null
    property string pluginName: ""

    readonly property string statusKey: {
        if (!item)
            return "";
        if (item.total > 0 && item.read >= item.total)
            return "done";
        if (item.hasNew)
            return "new";
        if (item.read > 0 && item.total > 0)
            return "reading";
        return "unread";
    }
    readonly property var statusStyle: ({
        "done": { "label": "DONE", "fg": "#0b2e1a", "bg": "#22c55e" },
        "new": { "label": "NEW", "fg": "#2a1a05", "bg": "#ffa332" },
        "reading": { "label": "READING", "fg": "#0b1f2e", "bg": "#60a5fa" },
        "unread": { "label": "UNREAD", "fg": "#e5e5e5", "bg": "#1f1f1f" }
    })

    signal activated(var item)

    radius: Theme.radius
    color: Theme.card
    border.width: mouse.containsMouse ? 1 : 0
    border.color: Theme.accent

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 8
        spacing: 6

        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: width / Theme.defaultRatio
            radius: 5
            clip: true
            color: Theme.field

            Image {
                id: cover
                anchors.fill: parent
                source: card.item ? card.item.cover : ""
                sourceSize.width: parent.width * 2
                asynchronous: true
                cache: true
                fillMode: Image.PreserveAspectCrop
                visible: status === Image.Ready
            }

            Label {
                anchors.centerIn: parent
                visible: cover.status === Image.Error || (card.item && card.item.cover === "")
                text: cover.status === Image.Error ? "cannot decode" : "no cover"
                color: Theme.textFaint
                font.pixelSize: 14
            }

            Rectangle {
                visible: card.item ? card.item.hasNew : false
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.margins: 6
                width: 10
                height: 10
                radius: 5
                color: Theme.accent
                border.width: 2
                border.color: Theme.card
            }

            Rectangle {
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right
                height: 44
                gradient: Gradient {
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop { position: 1; color: "#cc000000" }
                }
            }

            Row {
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.margins: 6
                spacing: 6

                Rectangle {
                    visible: card.statusKey !== ""
                    width: statusLabel.implicitWidth + 10
                    height: statusLabel.implicitHeight + 4
                    radius: 3
                    color: card.statusStyle[card.statusKey].bg
                    border.width: card.statusKey === "unread" ? 1 : 0
                    border.color: "#3a3a3a"

                    Label {
                        id: statusLabel
                        anchors.centerIn: parent
                        text: card.statusKey === "" ? "" : card.statusStyle[card.statusKey].label
                        color: card.statusKey === "" ? Theme.text : card.statusStyle[card.statusKey].fg
                        font.pixelSize: 10
                        font.bold: true
                    }
                }

                Rectangle {
                    visible: card.pluginName !== ""
                    width: srcLabel.implicitWidth + 10
                    height: srcLabel.implicitHeight + 4
                    radius: 3
                    color: "#cc000000"
                    border.width: 1
                    border.color: "#55ffffff"

                    Label {
                        id: srcLabel
                        anchors.centerIn: parent
                        text: card.pluginName
                        color: "#e5e5e5"
                        font.pixelSize: 10
                        elide: Text.ElideRight
                        width: Math.min(implicitWidth, 110)
                    }
                }
            }
        }

        Label {
            Layout.fillWidth: true
            text: card.item ? card.item.title : ""
            color: Theme.text
            font.pixelSize: 13
            font.bold: true
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        Item { Layout.fillHeight: true }

        Rectangle {
            Layout.fillWidth: true
            height: 3
            radius: 1.5
            color: Theme.line

            Rectangle {
                width: card.item && card.item.total > 0
                       ? parent.width * Math.min(card.item.read / card.item.total, 1)
                       : 0
                height: parent.height
                radius: parent.radius
                color: Theme.accent
            }
        }

        Label {
            text: card.item ? (card.item.total > 0 ? card.item.read + " / " + card.item.total : "unread") : ""
            color: Theme.textFaint
            font.pixelSize: 11
        }
    }

    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        cursorShape: Qt.PointingHandCursor
        onClicked: card.activated(card.item)
    }
}
