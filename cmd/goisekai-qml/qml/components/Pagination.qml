import ".."
import QtQuick
import QtQuick.Controls

// Prev/next plus a sliding window of page numbers. Always shows first and last
// so a 20+ page library stays navigable.
Row {
    id: pag

    property int page: 1
    property int pageCount: 1
    // First, last, current +/-1, with -1 marking an ellipsis on each side.
    readonly property var pages: {
        const out = [];
        const n = pag.pageCount;
        if (n <= 7) {
            for (let i = 1; i <= n; ++i) out.push(i)
            return out;
        }
        out.push(1);
        const from = Math.max(2, pag.page - 1);
        const to = Math.min(n - 1, pag.page + 1);
        if (from > 2)
            out.push(-1);

        for (let i = from; i <= to; ++i) out.push(i)
        if (to < n - 1)
            out.push(-1);

        out.push(n);
        return out;
    }

    signal requested(int page)

    spacing: 4

    NavButton {
        label: "‹ Prev"
        enabled: pag.page > 1
        onClicked: pag.requested(pag.page - 1)
    }

    Repeater {
        model: pag.pages

        delegate: NavButton {
            required property int modelData

            label: modelData < 0 ? "…" : String(modelData)
            isEllipsis: modelData < 0
            active: modelData === pag.page
            onClicked: {
                if (modelData > 0)
                    pag.requested(modelData);

            }
        }

    }

    NavButton {
        label: "Next ›"
        enabled: pag.page < pag.pageCount
        onClicked: pag.requested(pag.page + 1)
    }

    component NavButton: Button {
        id: navButton

        property string label: ""
        property bool active: false
        property bool isEllipsis: false

        enabled: !isEllipsis
        implicitWidth: isEllipsis ? 24 : Math.max(32, label.length * 9 + 16)
        implicitHeight: 28
        padding: 0

        background: Rectangle {
            radius: 5
            color: navButton.active ? "#3a2a10" : Theme.card
            border.width: 1
            border.color: navButton.active ? Theme.accent : "transparent"
            opacity: navButton.enabled ? 1 : 0.4
        }

        contentItem: Label {
            text: navButton.label
            color: navButton.active ? Theme.accent : (navButton.enabled ? Theme.textDim : Theme.textFaint)
            font.pixelSize: 11
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }

    }

}
