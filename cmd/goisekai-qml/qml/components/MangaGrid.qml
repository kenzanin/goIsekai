import ".."
import QtQuick
import QtQuick.Controls

// The grid itself. Slices Main.qml's filtered rows to one page; the GridView
// stays virtualized so only visible covers load.
GridView {
    id: view

    property var rows: []
    property int page: 1
    property int pageSize: 24
    property int totalRows: 0
    property string queryText: ""
    // Covers are asked for at cell resolution, so no 720px original is decoded
    // for a 168px card.
    readonly property int cellW: 168
    readonly property int cellH: 300

    signal activated(var item)

    function activateCurrent() {
        if (currentIndex < 0 || currentIndex >= count)
            return ;

        const start = Math.max(0, (view.page - 1) * view.pageSize);
        view.activated(view.rows[start + currentIndex]);
    }

    cellWidth: cellW
    cellHeight: cellH
    clip: true
    focus: true
    boundsBehavior: Flickable.StopAtBounds
    // Only the current page goes into the model; behind it sits a slice, not a
    // second copy of the library.
    model: {
        const start = Math.max(0, (view.page - 1) * view.pageSize);
        return view.rows.slice(start, start + view.pageSize);
    }
    Keys.onReturnPressed: view.activateCurrent()
    Keys.onEnterPressed: view.activateCurrent()

    // Two distinct empty states: a filter that matched nothing, and a library
    // that is genuinely empty (or failed to load).
    Label {
        anchors.centerIn: parent
        width: view.width - 64
        horizontalAlignment: Text.AlignHCenter
        wrapMode: Text.WordWrap
        visible: view.count === 0
        color: Theme.textDim
        font.pixelSize: 14
        text: view.totalRows === 0 ? statusText : "No manga matches “" + view.queryText.trim() + "” with these filters"
    }

    ScrollBar.vertical: ScrollBar {
        policy: ScrollBar.AsNeeded
    }

    delegate: MangaCard {
        width: view.cellW
        height: view.cellH
        item: modelData
        onActivated: (it) => {
            view.currentIndex = index;
            view.forceActiveFocus();
            view.activated(it);
        }
    }

}
