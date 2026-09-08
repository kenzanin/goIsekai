-- layouts/blank.lua
-- Minimal page shell (no nav). Used by the reader view.
-- Replaces layouts/blank.jet.
-- Called as: blank(data, content) -> string

return function(data, content)
    return table.concat({
        '<!DOCTYPE html>',
        '<html lang="en" class="h-full">',
        '<head>',
        '    <meta charset="utf-8">',
        '    <meta name="viewport" content="width=device-width, initial-scale=1">',
        '    <title>goIsekai</title>',
        '    <link rel="icon" type="image/svg+xml" href="/static/favicon.svg">',
        '    <link rel="stylesheet" href="/static/lib/tailwind.css">',
        '</head>',
        '<body class="bg-neutral-950 text-neutral-100 min-h-full">',
        '    <main id="content">',
        content,
        '    </main>',
        '    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.14.9/dist/cdn.min.js"></script>',
        '    <script defer src="/static/lib/alpine-components.js"></script>',
        '</body>',
        '</html>',
    }, '\n')
end
