import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

// One library card, matching partials/library_cards.lua: cover with the plugin
// badge bottom-left, two-line title, then "read / total" with a New or Done
// pill. Emits the item; Main.qml decides what a click does.
Rectangle {
    id: card

    property var item: null
    property string pluginName: ""
    readonly property bool done: item ? (item.total > 0 && item.read >= item.total) : false

    signal activated(var item)

    radius: Theme.radius
    color: Theme.card
    border.width: mouse.containsMouse ? 1 : 0
    border.color: Theme.accent

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // Cover. Fills whatever height the cell leaves after the title block, so
        // every card is identical and the whole thing scales with the window.
        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            radius: Theme.radius
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
                font.pixelSize: Theme.fsSmall
            }

            // Scrim so the plugin pill stays readable over light artwork.
            Rectangle {
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.right: parent.right
                height: 40

                gradient: Gradient {
                    GradientStop {
                        position: 0
                        color: "transparent"
                    }

                    GradientStop {
                        position: 1
                        color: "#cc000000"
                    }

                }

            }

            // Status overlay (status: ongoing/done) di cover — status manga dari plugin.
            Rectangle {
                visible: card.item && card.item.total > 0
                anchors.top: parent.top
                anchors.left: parent.left
                anchors.margins: 4
                width: statusLabel.implicitWidth + 8
                height: statusLabel.implicitHeight + 4
                radius: height / 2
                color: card.done ? "#2234c759" : "#226366f1"

                Label {
                    id: statusLabel

                    anchors.centerIn: parent
                    text: card.done ? "Done" : "Ongoing"
                    color: card.done ? "#34c759" : "#6366f1"
                    font.pixelSize: 12
                    font.bold: true
                }

            }

            // Plugin badge, bottom-left, like the web card's pluginBadge.
            Rectangle {
                visible: card.pluginName !== ""
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.margins: 8
                width: Math.min(pluginLabel.implicitWidth + 16, parent.width - 16)
                height: pluginLabel.implicitHeight + 6
                radius: height / 2
                color: "#99000000"

                Label {
                    id: pluginLabel

                    anchors.centerIn: parent
                    text: card.pluginName
                    color: "#f5f5f5"
                    font.pixelSize: Theme.fsSmall
                    elide: Text.ElideRight
                    width: Math.min(implicitWidth, card.width - 40)
                }

            }

        }

        // Title + progress.
        ColumnLayout {
            Layout.fillWidth: true
            Layout.margins: 10
            spacing: 4

            Label {
                Layout.fillWidth: true
                text: card.item ? card.item.title : ""
                color: Theme.text
                font.pixelSize: Theme.fsBody
                font.bold: true
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                elide: Text.ElideRight
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: 6

                Label {
                    text: card.item ? String(card.item.read) : "0"
                    color: Theme.accentSoft
                    font.pixelSize: Theme.fsSmall
                    font.bold: true
                }

                Label {
                    text: "/"
                    color: Theme.textFaint
                    font.pixelSize: Theme.fsSmall
                }

                Label {
                    text: card.item ? String(card.item.total) : "0"
                    color: Theme.text
                    font.pixelSize: Theme.fsSmall
                }

                Item {
                    Layout.fillWidth: true
                }

                // New wins over Done: a finished manga with a new chapter still
                // needs the reader's attention, same as the web card.
                Rectangle {
                    visible: card.item ? card.item.hasNew : false
                    width: newLabel.implicitWidth + 8
                    height: newLabel.implicitHeight + 3
                    radius: height / 2
                    color: Theme.newBadge

                    Label {
                        id: newLabel

                        anchors.centerIn: parent
                        text: "New"
                        color: "white"
                        font.pixelSize: 12
                        font.bold: true
                    }

                }

                Rectangle {
                    visible: card.done && !(card.item ? card.item.hasNew : false)
                    width: doneLabel.implicitWidth + 8
                    height: doneLabel.implicitHeight + 3
                    radius: height / 2
                    color: Theme.doneBadge

                    Label {
                        id: doneLabel

                        anchors.centerIn: parent
                        text: "Done"
                        color: "white"
                        font.pixelSize: 12
                        font.bold: true
                    }

                }

            }

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
