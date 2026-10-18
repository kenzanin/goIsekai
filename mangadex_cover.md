Berdasarkan dokumentasi MangaDex API, berikut cara mendapatkan cover/art manga:

## 📌 Cara Kerja API Cover MangaDex

Untuk mendapatkan cover, kamu butuh 2 hal:
1. **Manga ID**
2. **Cover filename** 

URL format cover:
- **Full size**: `https://uploads.mangadex.org/covers/:manga-id/:cover-filename`
- **Thumbnail 512px**: `https://uploads.mangadex.org/covers/:manga-id/:cover-filename.512.jpg`
- **Thumbnail 256px**: `https://uploads.mangadex.org/covers/:manga-id/:cover-filename.256.jpg`

---

## 💻 Contoh Code

### **1. JavaScript/Node.js (Fetch API)**

```javascript
// Cara 1: Langsung pakai URL cover (kalau sudah tahu manga ID & filename)
async function getMangaCover(mangaId, coverFilename) {
  const coverUrl = `https://uploads.mangadex.org/covers/${mangaId}/${coverFilename}`;
  return coverUrl;
}

// Cara 2: Ambil cover dari manga ID (recommended)
async function getMangaCoverById(mangaId) {
  try {
    // Step 1: Ambil data manga
    const mangaResponse = await fetch(`https://api.mangadex.org/manga/${mangaId}`);
    const mangaData = await mangaResponse.json();
    
    // Step 2: Cari cover_art relationship
    const coverRelationship = mangaData.data.relationships.find(
      rel => rel.type === 'cover_art'
    );
    
    if (!coverRelationship) {
      throw new Error('Cover not found');
    }
    
    const coverId = coverRelationship.id;
    
    // Step 3: Ambil detail cover untuk dapat filename
    const coverResponse = await fetch(`https://api.mangadex.org/cover/${coverId}`);
    const coverData = await coverResponse.json();
    
    const fileName = coverData.data.attributes.fileName;
    
    // Step 4: Buat URL cover
    const coverUrl = `https://uploads.mangadex.org/covers/${mangaId}/${fileName}`;
    const coverUrl512 = `https://uploads.mangadex.org/covers/${mangaId}/${fileName}.512.jpg`;
    const coverUrl256 = `https://uploads.mangadex.org/covers/${mangaId}/${fileName}.256.jpg`;
    
    return {
      full: coverUrl,
      thumbnail512: coverUrl512,
      thumbnail256: coverUrl256
    };
    
  } catch (error) {
    console.error('Error:', error);
    throw error;
  }
}

// Contoh penggunaan:
getMangaCoverById('8f3e1818-a015-491d-bd81-3addc4d7d56a')
  .then(covers => {
    console.log('Full size:', covers.full);
    console.log('512px:', covers.thumbnail512);
    console.log('256px:', covers.thumbnail256);
  });
```

---

### **2. Python (Requests)**

```python
import requests

def get_manga_cover(manga_id):
    """
    Mendapatkan URL cover manga dari MangaDex
    """
    try:
        # Step 1: Ambil data manga
        manga_url = f'https://api.mangadex.org/manga/{manga_id}'
        manga_response = requests.get(manga_url)
        manga_response.raise_for_status()
        manga_data = manga_response.json()
        
        # Step 2: Cari cover_art relationship
        cover_relationship = None
        for rel in manga_data['data']['relationships']:
            if rel['type'] == 'cover_art':
                cover_relationship = rel
                break
        
        if not cover_relationship:
            raise Exception('Cover not found')
        
        cover_id = cover_relationship['id']
        
        # Step 3: Ambil detail cover
        cover_url = f'https://api.mangadex.org/cover/{cover_id}'
        cover_response = requests.get(cover_url)
        cover_response.raise_for_status()
        cover_data = cover_response.json()
        
        file_name = cover_data['data']['attributes']['fileName']
        
        # Step 4: Buat URL cover
        base_cover_url = f'https://uploads.mangadex.org/covers/{manga_id}/{file_name}'
        
        return {
            'full': base_cover_url,
            'thumbnail_512': f'{base_cover_url}.512.jpg',
            'thumbnail_256': f'{base_cover_url}.256.jpg'
        }
        
    except requests.exceptions.RequestException as e:
        print(f'Error: {e}')
        raise

# Contoh penggunaan:
if __name__ == '__main__':
    manga_id = '8f3e1818-a015-491d-bd81-3addc4d7d56a'
    covers = get_manga_cover(manga_id)
    
    print('Full size:', covers['full'])
    print('512px:', covers['thumbnail_512'])
    print('256px:', covers['thumbnail_256'])
```

---

### **3. Python dengan Reference Expansion (Lebih Efisien)**

```python
import requests

def get_manga_cover_fast(manga_id):
    """
    Mendapatkan cover dengan reference expansion (1 request saja)
    """
    try:
        # Ambil manga dengan include cover_art
        url = f'https://api.mangadex.org/manga/{manga_id}'
        params = {
            'includes[]': 'cover_art'  # Reference expansion
        }
        
        response = requests.get(url, params=params)
        response.raise_for_status()
        data = response.json()
        
        # Cari cover dari relationships
        for rel in data['data']['relationships']:
            if rel['type'] == 'cover_art':
                file_name = rel['attributes']['fileName']
                
                return {
                    'full': f'https://uploads.mangadex.org/covers/{manga_id}/{file_name}',
                    'thumbnail_512': f'https://uploads.mangadex.org/covers/{manga_id}/{file_name}.512.jpg',
                    'thumbnail_256': f'https://uploads.mangadex.org/covers/{manga_id}/{file_name}.256.jpg'
                }
        
        raise Exception('Cover not found')
        
    except requests.exceptions.RequestException as e:
        print(f'Error: {e}')
        raise

# Contoh penggunaan:
covers = get_manga_cover_fast('8f3e1818-a015-491d-bd81-3addc4d7d56a')
print(covers)
```

---

### **4. Download Cover Image**

```python
import requests
import uuid

def download_manga_cover(manga_id, output_path='cover.jpg'):
    """
    Download dan simpan cover manga
    """
    covers = get_manga_cover_fast(manga_id)
    
    # Download gambar
    response = requests.get(covers['full'])
    response.raise_for_status()
    
    # Simpan ke file
    with open(output_path, 'wb') as f:
        f.write(response.content)
    
    print(f'Cover berhasil disimpan di {output_path}')

# Contoh:
download_manga_cover('8f3e1818-a015-491d-bd81-3addc4d7d56a', 'my-manga-cover.jpg')
```

---

## 📚 Endpoint API yang Digunakan:

1. **GET `/manga/{id}`** - Ambil data manga [[1]]
2. **GET `/cover/{id}`** - Ambil detail cover [[1]]
3. **GET `/cover`** - List semua cover (pakai query `manga[]`) [[3]]

## 🔗 Resources:
- Dokumentasi lengkap: https://api.mangadex.org/docs/03-manga/covers/ [[1]]
- Swagger API: https://api.mangadex.org/docs/swagger.html [[2]]

Semoga membantu! 🎉