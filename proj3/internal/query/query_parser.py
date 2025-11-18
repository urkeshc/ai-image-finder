import json
import sys
import openai

# If you install openai==0.28.0, this code continues to work:
openai.api_key = ""

def main():
    # Template for metadata fields, used in initial prompt
    metadata_template = {
        "photo_submitted_at": None,
        "photo_featured": None,
        "photo_width": None,
        "photo_height": None,
        "photo_aspect_ratio": None,
        "photo_description": None,
        "photographer_username": None,
        "photographer_first_name": None,
        "photographer_last_name": None,
        "exif_camera_make": None,
        "exif_camera_model": None,
        "year": None,
        "month": None,
        "day": None,
        "exif_iso": None,
        "exif_aperture_value": None,
        "exif_focal_length": None,
        "exif_exposure_time": None,
        "photo_location_name": None,
        "photo_location_latitude": None,
        "photo_location_longitude": None,
        "photo_location_country": None,
        "photo_location_city": None
    }

    # Check if we are in reprompt mode (sys.argv[1] = prev_metadata_json, sys.argv[2] = new_user_input)
    if len(sys.argv) == 3: # script_name, prev_metadata_json, new_user_input
        previous_metadata_json_str = sys.argv[1]
        new_user_input_str = sys.argv[2]
        
        prompt_text = f"""
Here is the current JSON object of photo metadata:
{previous_metadata_json_str}

The user has provided this new information:
"{new_user_input_str}"

Please update the JSON object above based on the new information.
If the new information provides a value for a field, use it.
If the new information implies a field should be null (e.g., "I forgot the year"), set it to null.
Otherwise, preserve existing non-null values from the provided JSON object.
For the 'photo_location_country' field, if a country is identified by an alias (e.g., "USA", "UK"), please use its canonical name (e.g., "United States", "United Kingdom").
Return *only* the updated JSON object (the metadata map itself), not wrapped in any other structure.
        """
    elif len(sys.argv) == 2: # script_name, user_input
        user_input = sys.argv[1]
        prompt_text = f"""
User input: "{user_input}"
Based on the user input, please generate a JSON object with the following photo metadata fields:
{json.dumps(metadata_template, indent=2)}

Infer values where possible. If a field cannot be determined, use null.
You are allowed to infer values based on context, for example, if a country or city is provided, you can impute a random latitude and longitude in that area. 
However, do not invent values if you have no clue; leave them as null unless you are 99% sure of your guess.
For the 'photo_location_country' field, if a country is identified by an alias (e.g., "USA", "UK"), please use its canonical name (e.g., "United States", "United Kingdom").
Return *only* the JSON object (the metadata map itself), not wrapped in any other structure.
        """
    else: # No input or incorrect number of arguments
        user_input = input("Please enter your search query: ")
        prompt_text = f"""
User input: "{user_input}"
Based on the user input, please generate a JSON object with the following photo metadata fields:
{json.dumps(metadata_template, indent=2)}

Infer values where possible. If a field cannot be determined, use null.
For the 'photo_location_country' field, if a country is identified by an alias (e.g., "USA", "UK"), please use its canonical name (e.g., "United States", "United Kingdom").
Return *only* the JSON object (the metadata map itself), not wrapped in any other structure.
        """
    
    response = openai.ChatCompletion.create(
        model="o3-mini",
        messages=[
            {"role": "system", "content": "You are a helpful assistant. Do not wrap your JSON in triple backticks or code fences. Return valid JSON only."},
            {"role": "user", "content": prompt_text}
        ]
    )
    
    # Calculate cost
    usage = response.get("usage", {})
    prompt_tokens = usage.get("prompt_tokens", 0)
    completion_tokens = usage.get("completion_tokens", 0)
    # o3-mini cost: 1.1 dollars per million tokens
    # o1 cost: 15 dollars per million tokens
    # o3 cost: 10 dollars per million tokens 
    cost = (prompt_tokens + completion_tokens) / 1000000.0 * 1.1 
    
    
    result_text = response.choices[0].message.content
    
    # Try to parse the response as JSON (this should be the flat metadata map)
    try:
        metadata_fields_map = json.loads(result_text)
    except json.JSONDecodeError:
        metadata_fields_map = {"error": "Failed to parse response as JSON", "raw_text": result_text}
    
    # Wrap the metadata_fields_map into the standard Query structure
    result = {
        "message": f"Extracted metadata from GPT (cost: ${cost:.6f})",
        "metadata": metadata_fields_map 
    }
    
    # Make sure this is the ONLY print statement in the whole script
    print(json.dumps(result))

if __name__ == "__main__":
    main()