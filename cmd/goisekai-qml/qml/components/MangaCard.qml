import ".."
import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One library card. Emits the item rather than navigating, so Main.qml stays the
// only place that decides what a click does.
Rectangle {
    id: card

    property var item: null
    property real thumbRatio: Theme.defaultRatio

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
            Layout.preferredHeight: width / card.thumbRatio
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
                // Loading and error must look different: a missing Qt image
                // plugin (kimageformats for AVIF) fails every cover, and the
                // user needs to tell that from "still fetching".
                visible: status === Image.Ready
            }

            Label {
                anchors.centerIn: parent
                visible: cover.status === Image.Error || (card.item && card.item.cover === "")
                text: cover.status === Image.Error ? "cannot decode" : "no cover"
                color: Theme.textFaint
                font.pixelSize: 17
            }

            // New-chapter badge, from the API's has_new.
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

        }

        Label {
            Layout.fillWidth: true
            text: card.item ? card.item.title : ""
            color: Theme.text
            font.pixelSize: 17
            font.bold: true
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        Item {
            Layout.fillHeight: true
        }

        Rectangle {
            Layout.fillWidth: true
            height: 3
            radius: 1.5
            color: Theme.line

            Rectangle {
                width: card.item && card.item.total > 0 ? parent.width * Math.min(card.item.read / card.item.total, 1) : 0
                height: parent.height
                radius: parent.radius
                color: Theme.accent
            }

        }

        Label {
            text: card.item ? (card.item.total > 0 ? card.item.read + " / " + card.item.total : "unread") : ""
            color: Theme.textFaint
            font.pixelSize: 17
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
