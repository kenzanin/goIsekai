Berdasarkan dokumentasi MangaDex API, berikut cara mendapatkan semua data tersebut dalam **satu query** menggunakan **Reference Expansion** [[1]][[4]]:

## 📊 Relationship Types yang Tersedia

Dari dokumentasi, relationship types yang bisa di-include adalah [[4]]:
- `author` - Author resource
- `artist` - Author resource (drawers only)
- `tag` - Tag resource (untuk genres)
- `manga` - Manga resource (untuk related manga)
- `cover_art` - Cover Art
- `scanlation_group` - ScanlationGroup resource
- `chapter` - Chapter resource
- `user` - User resource
- `custom_list` - CustomList resource

---

## 💻 Contoh Code Lengkap

### **1. JavaScript/Node.js**

```javascript
async function getMangaCompleteInfo(mangaId) {
  try {
    // Satu query dengan semua includes
    const url = new URL(`https://api.mangadex.org/manga/${mangaId}`);
    
    // Tambahkan semua includes yang dibutuhkan
    url.searchParams.append('includes[]', 'author');
    url.searchParams.append('includes[]', 'artist');
    url.searchParams.append('includes[]', 'tag');
    url.searchParams.append('includes[]', 'manga');
    url.searchParams.append('includes[]', 'cover_art');
    
    const response = await fetch(url);
    const data = await response.json();
    
    const manga = data.data;
    const attributes = manga.attributes;
    const relationships = manga.relationships;
    
    // 1. SINOPSIS
    const synopsis = attributes.description?.en || 'No synopsis available';
    
    // 2. ALTERNATIVE TITLES
    const altTitles = attributes.altTitles || [];
    const altTitlesFlat = altTitles.map(title => 
      Object.values(title)[0]
    ).join(', ');
    
    // 3. GENRES (dari tags)
    const tags = relationships.filter(rel => rel.type === 'tag');
    const genres = tags.map(tag => {
      const tagName = tag.attributes?.name?.en || 'Unknown';
      const isGenre = tag.attributes?.group === 'genre';
      return isGenre ? tagName : null;
    }).filter(Boolean);
    
    // 4. AUTHORS & ARTISTS
    const authors = relationships
      .filter(rel => rel.type === 'author')
      .map(author => author.attributes?.name || 'Unknown');
    
    const artists = relationships
      .filter(rel => rel.type === 'artist')
      .map(artist => artist.attributes?.name || 'Unknown');
    
    // 5. RELATED MANGA
    const relatedManga = relationships
      .filter(rel => rel.type === 'manga')
      .map(rel => ({
        id: rel.id,
        title: rel.attributes?.title?.en || 'Unknown',
        relation: rel.attributes?.relation || 'unknown'
      }));
    
    // 6. COVER ART
    const coverArt = relationships.find(rel => rel.type === 'cover_art');
    const coverFileName = coverArt?.attributes?.fileName;
    const coverUrl = coverFileName 
      ? `https://uploads.mangadex.org/covers/${mangaId}/${coverFileName}`
      : null;
    
    return {
      id: manga.id,
      title: attributes.title?.en || attributes.title?.ja || 'No title',
      synopsis,
      alternativeTitles: altTitlesFlat,
      genres,
      authors,
      artists,
      relatedManga,
      coverUrl,
      status: attributes.status,
      year: attributes.year,
      demographic: attributes.publicationDemographic
    };
    
  } catch (error) {
    console.error('Error:', error);
    throw error;
  }
}

// Contoh penggunaan:
getMangaCompleteInfo('f9c33607-9180-4ba6-b85c-e4b5faee7192')
  .then(manga => {
    console.log('=== MANGA INFO ===');
    console.log('Title:', manga.title);
    console.log('Synopsis:', manga.synopsis);
    console.log('Alt Titles:', manga.alternativeTitles);
    console.log('Genres:', manga.genres.join(', '));
    console.log('Authors:', manga.authors.join(', '));
    console.log('Artists:', manga.artists.join(', '));
    console.log('Related Manga:', manga.relatedManga);
    console.log('Cover:', manga.coverUrl);
  });
```

---

### **2. Python**

```python
import requests

def get_manga_complete_info(manga_id):
    """
    Mendapatkan semua informasi manga dalam satu query
    """
    try:
        # Satu query dengan semua includes
        url = f'https://api.mangadex.org/manga/{manga_id}'
        params = {
            'includes[]': ['author', 'artist', 'tag', 'manga', 'cover_art']
        }
        
        response = requests.get(url, params=params)
        response.raise_for_status()
        data = response.json()
        
        manga = data['data']
        attributes = manga['attributes']
        relationships = manga['relationships']
        
        # 1. SINOPSIS
        synopsis = attributes.get('description', {}).get('en', 'No synopsis available')
        
        # 2. ALTERNATIVE TITLES
        alt_titles = attributes.get('altTitles', [])
        alt_titles_flat = ', '.join([
            list(title.values())[0] for title in alt_titles if title
        ])
        
        # 3. GENRES (dari tags)
        tags = [rel for rel in relationships if rel['type'] == 'tag']
        genres = []
        for tag in tags:
            tag_attrs = tag.get('attributes', {})
            if tag_attrs.get('group') == 'genre':
                genre_name = tag_attrs.get('name', {}).get('en', 'Unknown')
                genres.append(genre_name)
        
        # 4. AUTHORS & ARTISTS
        authors = [
            rel.get('attributes', {}).get('name', 'Unknown')
            for rel in relationships if rel['type'] == 'author'
        ]
        
        artists = [
            rel.get('attributes', {}).get('name', 'Unknown')
            for rel in relationships if rel['type'] == 'artist'
        ]
        
        # 5. RELATED MANGA
        related_manga = [
            {
                'id': rel['id'],
                'title': rel.get('attributes', {}).get('title', {}).get('en', 'Unknown'),
                'relation': rel.get('attributes', {}).get('relation', 'unknown')
            }
            for rel in relationships if rel['type'] == 'manga'
        ]
        
        # 6. COVER ART
        cover_art = next((rel for rel in relationships if rel['type'] == 'cover_art'), None)
        cover_file_name = cover_art.get('attributes', {}).get('fileName') if cover_art else None
        cover_url = f'https://uploads.mangadex.org/covers/{manga_id}/{cover_file_name}' if cover_file_name else None
        
        return {
            'id': manga['id'],
            'title': attributes.get('title', {}).get('en') or attributes.get('title', {}).get('ja', 'No title'),
            'synopsis': synopsis,
            'alternative_titles': alt_titles_flat,
            'genres': genres,
            'authors': authors,
            'artists': artists,
            'related_manga': related_manga,
            'cover_url': cover_url,
            'status': attributes.get('status'),
            'year': attributes.get('year'),
            'demographic': attributes.get('publicationDemographic')
        }
        
    except requests.exceptions.RequestException as e:
        print(f'Error: {e}')
        raise

# Contoh penggunaan:
if __name__ == '__main__':
    manga_id = 'f9c33607-9180-4ba6-b85c-e4b5faee7192'
    manga = get_manga_complete_info(manga_id)
    
    print('=== MANGA INFO ===')
    print(f"Title: {manga['title']}")
    print(f"Synopsis: {manga['synopsis']}")
    print(f"Alt Titles: {manga['alternative_titles']}")
    print(f"Genres: {', '.join(manga['genres'])}")
    print(f"Authors: {', '.join(manga['authors'])}")
    print(f"Artists: {', '.join(manga['artists'])}")
    print(f"Related Manga: {manga['related_manga']}")
    print(f"Cover: {manga['cover_url']}")
```

---

## 📝 Penjelasan Struktur Data

### **Data yang Langsung Tersedia (tanpa include):**
- ✅ **Synopsis** → `attributes.description`
- ✅ **Alternative Titles** → `attributes.altTitles`
- ✅ **Main Title** → `attributes.title`
- ✅ **Status, Year, Demographic** → `attributes.*`

### **Data yang Perlu Include:**
- 📌 **Authors** → `includes[]=author`
- 📌 **Artists** → `includes[]=artist`
- 📌 **Genres/Tags** → `includes[]=tag`
- 📌 **Related Manga** → `includes[]=manga`
- 📌 **Cover Art** → `includes[]=cover_art`

---

## 🔗 URL Request Example

```
https://api.mangadex.org/manga/{id}?includes[]=author&includes[]=artist&includes[]=tag&includes[]=manga&includes[]=cover_art
```

Dengan cara ini, kamu hanya perlu **1 request** untuk mendapatkan semua data yang dibutuhkan! 🎉

**Referensi:** Dokumentasi Reference Expansion MangaDex [[1]][[4]]