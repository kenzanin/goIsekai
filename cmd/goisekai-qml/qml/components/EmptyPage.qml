import ".."
import QtQuick
import QtQuick.Controls

// Placeholder for the pages that are not built yet. Saying which page it is
// beats a blank rectangle: an empty screen reads as a bug.
Rectangle {
    id: page

    property string sectionTitle: ""

    color: Theme.bg

    Column {
        anchors.centerIn: parent
        spacing: 10

        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: page.sectionTitle
            color: Theme.text
            font.pixelSize: 22
            font.bold: true
        }

        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: "Not built yet — the library tab is the finished page."
            color: Theme.textFaint
            font.pixelSize: 17
        }

    }

}
