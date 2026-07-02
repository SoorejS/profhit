import os
import re

html_files = [f for f in os.listdir('.') if f.endswith('.html')]

# The standard core CSS block we want in every file
core_css_replacement = """    <link rel="stylesheet" href="css/tokens.css">
    <link rel="stylesheet" href="css/base.css">
    <link rel="stylesheet" href="css/layout.css">
    <link rel="stylesheet" href="css/components.css">
    <link rel="stylesheet" href="css/utilities.css">"""

for file in html_files:
    with open(file, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Replace the block of tokens + base + components with the expanded list
    content = re.sub(
        r'<link rel="stylesheet" href="css/tokens\.css">\s*<link rel="stylesheet" href="css/base\.css">\s*<link rel="stylesheet" href="css/components\.css">',
        core_css_replacement,
        content
    )
    
    # Also update page specific CSS links to point to css/pages/
    content = re.sub(r'href="css/(landing|auth|dashboard|admin)\.css"', r'href="css/pages/\1.css"', content)
    
    with open(file, 'w', encoding='utf-8') as f:
        f.write(content)

print("Updated HTML CSS references.")
