-- partials/toast.lua
-- Alpine.js mount points for the toast region and confirm modal.
-- Replaces partials/toast.jet.
-- Called as: toast(data) -> string
-- The actual rendering logic lives in $store.toast / $store.confirm
-- (defined in /static/lib/alpine-components.js).

-- Class constants reused by the JS store when it builds toast items.
local TOAST_ITEM_BASE  = 'toast-item pointer-events-auto flex items-start gap-2.5 w-80 max-w-[calc(100vw-2rem)] bg-neutral-900 border border-neutral-700 rounded-lg shadow-xl p-3 pr-2.5 relative overflow-hidden'
local TOAST_ICON_BASE  = 'shrink-0 flex items-center justify-center size-5 rounded-full text-xs font-bold text-neutral-100'
local TOAST_TEXT       = 'text-sm text-neutral-200 leading-snug break-words'
local TOAST_CLOSE      = 'ml-auto shrink-0 size-5 inline-flex items-center justify-center rounded text-neutral-500 hover:text-neutral-300'
local TOAST_ACCENT_SUCCESS = 'bg-emerald-500'
local TOAST_ACCENT_ERROR   = 'bg-red-500'
local TOAST_ACCENT_INFO    = 'bg-indigo-400'

return function(data)
    return table.concat({
        -- Toast region: Alpine store renders items into this container.
        '<div id="toast-region" x-data x-init="$store.toast.render(this.$el)" class="fixed bottom-4 left-4 z-50 space-y-2"></div>',
        '',
        -- Toast item transition styles (reused by JS-injected items).
        '<style>',
        '  .toast-item { opacity: 0; transform: translateY(8px); transition: opacity .18s ease, transform .18s ease; }',
        '  .toast-item.toast-visible { opacity: 1; transform: translateY(0); }',
        '  .toast-item.toast-leave { opacity: 0; transform: translateY(8px); }',
        '</style>',
        '',
        -- Confirm modal mount point.
        '<div id="confirm-modal" x-data x-init="$store.confirm.mount(this.$el)" class="fixed inset-0 z-50 hidden items-center justify-center bg-black/50" style="display:none">',
        '    <div class="relative bg-neutral-900 border border-neutral-700 rounded-lg shadow-xl w-full max-w-sm p-5">',
        '        <p id="confirm-message" class="text-sm text-neutral-200 mb-4"></p>',
        '        <div class="flex justify-end gap-2">',
        '            <button type="button" id="confirm-cancel" class="border border-neutral-700 hover:bg-neutral-800 text-neutral-300 rounded-md px-3 py-1.5 text-sm">Cancel</button>',
        '            <button type="button" id="confirm-ok" class="bg-red-600 hover:bg-red-500 text-white rounded-md px-3 py-1.5 text-sm font-medium">Confirm</button>',
        '        </div>',
        '    </div>',
        '</div>',
    }, '\n')
end
