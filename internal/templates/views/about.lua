-- views/about.lua
-- About page: the project README rendered as HTML.
-- data.Content: pre-rendered HTML string (server-side markdown conversion).

return function(data)
	local body = [[<div class="max-w-3xl mx-auto about-doc">]]
	body = body .. (data.Content or "")
	body = body
		.. [[<hr style="margin:2rem 0;border-color:#262626;">
<p style="color:#a3a3a3;font-size:0.875rem;">Man in the loop: <a href="mailto:kenzanin@gmail.com" style="color:#818cf8;text-decoration:underline;">Kenzanin at Gmail dot com</a></p>
</div>
<style>
.about-doc h1 { font-size: 1.5rem; font-weight: 600; margin: 1.5rem 0 0.75rem; }
.about-doc > h1:first-child { margin-top: 0; }
.about-doc h2 { font-size: 1.25rem; font-weight: 600; margin: 1.75rem 0 0.625rem; padding-bottom: 0.25rem; border-bottom: 1px solid #262626; }
.about-doc h3 { font-size: 1rem; font-weight: 600; margin: 1.25rem 0 0.5rem; }
.about-doc p { color: #d4d4d4; font-size: 0.875rem; line-height: 1.6; margin: 0.5rem 0; }
.about-doc ul { margin: 0.5rem 0; padding-left: 1.25rem; list-style: disc; }
.about-doc li { color: #d4d4d4; font-size: 0.875rem; line-height: 1.6; margin: 0.25rem 0; }
.about-doc ol { margin: 0.5rem 0; padding-left: 1.25rem; list-style: decimal; }
.about-doc pre { background: #171717; border: 1px solid #262626; border-radius: 0.5rem; padding: 0.75rem; overflow-x: auto; margin: 1rem 0; font-size: 0.75rem; line-height: 1.5; color: #d4d4d4; }
.about-doc code { font-family: ui-monospace, monospace; background: #262626; border-radius: 0.25rem; padding: 0.1rem 0.3rem; font-size: 0.8em; }
.about-doc pre code { background: none; padding: 0; }
.about-doc table { border-collapse: collapse; margin: 1rem 0; font-size: 0.875rem; width: 100%; }
.about-doc th, .about-doc td { border: 1px solid #262626; padding: 0.4rem 0.75rem; text-align: left; }
.about-doc th { background: #171717; font-weight: 500; }
.about-doc blockquote { border-left: 3px solid #404040; margin: 0.75rem 0; padding: 0.25rem 1rem; color: #a3a3a3; }
.about-doc hr { border-color: #262626; margin: 2rem 0; }
.about-doc a { color: #818cf8; text-decoration: underline; text-underline-offset: 2px; }
.about-doc a:hover { color: #a5b4fc; }
</style>]]
	return body
end
