import os
import re

html_files = [f for f in os.listdir('.') if f.endswith('.html')]

for file in html_files:
    with open(file, 'r', encoding='utf-8') as f:
        content = f.read()
    
    # Remove old api.js and components.js
    content = re.sub(r'<script src="js/api\.js"></script>\n?\s*', '', content)
    content = re.sub(r'<script src="js/components\.js"></script>\n?\s*', '', content)
    
    def replacer(match):
        script_name = match.group(1)
        if script_name in ['auth', 'dashboard', 'market', 'profile', 'wallet', 'admin']:
            return f'<script type="module" src="js/pages/{script_name}.js"></script>'
        return match.group(0)
    
    content = re.sub(r'<script src="js/([a-zA-Z0-9_]+)\.js"></script>', replacer, content)
    
    with open(file, 'w', encoding='utf-8') as f:
        f.write(content)

# Add sidebar/topbar imports to pages that use layout
pages_with_layout = ['dashboard.js', 'market.js', 'profile.js', 'wallet.js', 'admin.js']

for page in pages_with_layout:
    if os.path.exists(f'js/pages/{page}'):
        with open(f'js/pages/{page}', 'r', encoding='utf-8') as f:
            content = f.read()
        
        # Only add if not already there
        if "import '../components/sidebar.js';" not in content:
            imports = "import '../components/sidebar.js';\nimport '../components/topbar.js';\n"
            content = imports + content
            
        with open(f'js/pages/{page}', 'w', encoding='utf-8') as f:
            f.write(content)

print("HTML script tags and component imports updated.")
