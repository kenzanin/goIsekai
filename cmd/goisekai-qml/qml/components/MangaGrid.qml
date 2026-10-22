import ".."
import QtQuick
import QtQuick.Controls

// The grid itself. Slices Main.qml's filtered rows to one page; the GridView
// stays virtualized so only visible covers load.
GridView {
    id: view

    property var rows: []
    property int page: 1
    property int pageSize: 25
    property int totalRows: 0
    property string queryText: ""
    // Width-driven, not a fixed cell size: a fixed 168px cell fits six columns
    // at one window width and four at another, so "5 per row" only holds if the
    // cell is derived from the viewport.
    readonly property int columns: 5
    readonly property int cellW: Math.max(120, Math.floor(width / columns))
    readonly property int cellH: Math.round(cellW / Theme.defaultRatio) + 80

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
