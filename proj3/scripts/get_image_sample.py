import pandas as pd
import pathlib
import json
import requests
import sys

try:
    n_images = int(sys.argv[1])
except:
    n_images = 100

script_dir = pathlib.Path(__file__).resolve().parent
photos_path = script_dir.parent / "data" / "unsplash-research-dataset-lite-latest" / "photos.csv000"
photos = pd.read_csv(photos_path, sep='\t', nrows=n_images)
images_dir = script_dir.parent / "data" / "images"
metadata_dir = script_dir.parent / "data" / "metadata"
images_dir.mkdir(parents=True, exist_ok=True)
metadata_dir.mkdir(parents=True, exist_ok=True)

for _, row in photos.iterrows():
    img_path = images_dir / f"{row.photo_id}.jpg"
    meta_path = metadata_dir / f"{row.photo_id}.json"
    img_data = requests.get(row.photo_image_url, timeout=10).content
    img_path.write_bytes(img_data)
    
    # Convert row to dictionary with all available fields
    # Handle NaN values to make the data JSON serializable
    info = row.where(pd.notnull(row), None).to_dict()
    
    meta_path.write_text(json.dumps(info, indent=2))

print(f"✓ downloaded {len(photos)} images")