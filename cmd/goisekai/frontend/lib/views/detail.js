import { bindings } from "../bindings.js";
import { loadImage } from "../utils.js";
import { readerView } from "../reader.js";

// ── Manga detail component ────────────────────────────────────────
export const detailView = () => ({
  pluginID: '',
  mangaID: '',
  manga: null,
  chapters: [],
  inLibrary: false,
  loading: false,
  error: null,
  coverUrl: '',

  async load(pid, mid) {
    // Skip a re-entrant/competitive re-fire with the SAME target (the detail
    // x-effect re-runs on every reactive change and load() itself mutates
    // reactive state). Without this the re-fire blanks manga/coverUrl mid-load.
    if (this.loading && pid === this.pluginID && mid === this.mangaID) return;
    console.log('[detail] load:', 'pid=' + pid + ' mid=' + mid + ' hash=' + (window.location.hash || ''));
    this.pluginID = pid;
    this.mangaID = mid;
    this.loading = true;
    this.error = null;
    this.manga = null;
    this.chapters = [];
    this.coverUrl = '';

    try {
      const [manga, chapters] = await bindings.mangaDetails(pid, mid);
      this.manga = manga;
      this.chapters = Array.isArray(chapters)
        ? chapters.slice().sort((a, b) => (b.chapter_num || 0) - (a.chapter_num || 0))
        : [];

      if (manga.cover_url) {
        const url = await loadImage(pid, manga.cover_url);
        if (url) this.coverUrl = url;
      }

      // Expose chapters (ascending by number) to the reader for next-chapter nav.
      const asc = this.chapters.slice().sort((a, b) => (a.chapter_num || 0) - (b.chapter_num || 0));
      this.$store.app.chaptersByManga[pid + '|' + mid] = asc;

      // Check library membership
      if (!this.$store.app.libraryList) await this.$store.app.loadLibrary();
      this.inLibrary = (this.$store.app.libraryList || []).some(
        (m) => m.PluginID === pid && m.SourceMangaID === mid
      );
    } catch (err) {
      this.error = 'Failed to load manga details';
    } finally {
      this.loading = false;
    }
  },

  async toggleLibrary() {
    try {
      await bindings.toggleLibrary(this.pluginID, this.mangaID);
      await this.$store.app.loadLibrary();
      this.inLibrary = !this.inLibrary;
    } catch (err) {
      console.error('toggle library failed:', err);
    }
  },

  chapterUrl(ch) {
    return `#/read/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}/${encodeURIComponent(ch.id)}`;
  },

  readProgress(ch) {
    return this.$store.app.readProgress[ch.id] || null;
  },

  get chaptersAscending() {
    return this.chapters.slice().sort((a, b) => (a.chapter_num || 0) - (b.chapter_num || 0));
  },

  // Where the "Continue Reading" button points: first chapter, or the
  // furthest-read chapter (at its last page), or the next chapter once the
  // furthest-read one is complete.
  get startTarget() {
    const list = this.chaptersAscending;
    if (!list.length) return null;
    let best = null, bestPage = -1;
    for (const ch of list) {
      const p = this.readProgress(ch);
      if (p && p.lastPage > bestPage) { bestPage = p.lastPage; best = ch; }
    }
    if (!best) {
      return { chapter: list[0], page: 0, label: 'Start Reading' };
    }
    const p = this.readProgress(best);
    if (p.lastPage >= p.pageCount) {
      const idx = list.indexOf(best);
      const nextCh = list[idx + 1];
      if (nextCh) return { chapter: nextCh, page: 0, label: 'Continue Reading', num: nextCh.chapter_num };
      return null; // fully caught up
    }
    return { chapter: best, page: p.lastPage, label: 'Continue Reading', num: best.chapter_num };
  },

  startReading() {
    const t = this.startTarget;
    // Never build a read URL with an empty plugin/manga id (would produce a
    // broken #/read// or #/read/pid//cid that resets the reader to empty IDs).
    if (!t || !t.chapter || !this.pluginID || !this.mangaID) return;
    this.$store.app.navigate(
      `#/read/${encodeURIComponent(this.pluginID)}/${encodeURIComponent(this.mangaID)}` +
      `/${encodeURIComponent(t.chapter.id)}?page=${t.page}`
    );
  },
});
