#!/usr/bin/env python3
"""
Code Complexity Analyser for Inspectre
Analyses code complexity of Python, JavaScript, and Go files.
"""

import os
import sys
import json
import re
import time
from datetime import datetime
import fnmatch

# Configuration
max_complexity = int(os.environ.get('INSPECTRE_CONFIG_max_complexity', 10))
repo_path = sys.argv[1] if len(sys.argv) > 1 else '.'

# File patterns to analyse
file_patterns = {
    'python': ['*.py'],
    'javascript': ['*.js', '*.jsx', '*.ts', '*.tsx'],
    'go': ['*.go'],
}

# Regular expressions for complexity detection
complexity_patterns = {
    'python': {
        'function': r'def\s+\w+\s*\(',
        'class': r'class\s+\w+',
        'if': r'\s+if\s+',
        'else': r'\s+else:',
        'elif': r'\s+elif\s+',
        'for': r'\s+for\s+',
        'while': r'\s+while\s+',
        'try': r'\s+try:',
        'except': r'\s+except',
        'with': r'\s+with\s+',
    },
    'javascript': {
        'function': r'function\s+\w+\s*\(|const\s+\w+\s*=\s*\(.*\)\s*=>|let\s+\w+\s*=\s*\(.*\)\s*=>|var\s+\w+\s*=\s*\(.*\)\s*=>',
        'class': r'class\s+\w+',
        'if': r'\s+if\s*\(',
        'else': r'\s+else\s*{',
        'else if': r'\s+else\s+if\s*\(',
        'for': r'\s+for\s*\(',
        'while': r'\s+while\s*\(',
        'try': r'\s+try\s*{',
        'catch': r'\s+catch\s*\(',
        'switch': r'\s+switch\s*\(',
        'case': r'\s+case\s+',
    },
    'go': {
        'function': r'func\s+\w+\s*\(',
        'struct': r'type\s+\w+\s+struct',
        'interface': r'type\s+\w+\s+interface',
        'if': r'\s+if\s+',
        'else': r'\s+else\s*{',
        'else if': r'\s+else\s+if\s+',
        'for': r'\s+for\s+',
        'switch': r'\s+switch\s+',
        'case': r'\s+case\s+',
        'select': r'\s+select\s*{',
    },
}

def find_files(base_dir, patterns):
    """Find files matching patterns in base_dir."""
    matched_files = []
    
    for root, _, files in os.walk(base_dir):
        # Skip hidden directories and common excludes
        if any(part.startswith('.') or part in ['node_modules', 'vendor', '__pycache__'] 
               for part in root.split(os.sep)):
            continue
            
        for filename in files:
            filepath = os.path.join(root, filename)
            if any(fnmatch.fnmatch(filename, pattern) for pattern in patterns):
                matched_files.append(filepath)
                
    return matched_files

def calculate_complexity(filepath, language):
    """Calculate code complexity for a file."""
    try:
        with open(filepath, 'r', encoding='utf-8', errors='ignore') as f:
            content = f.read()
    except Exception as e:
        print(f"Error reading {filepath}: {e}", file=sys.stderr)
        return {}
        
    stats = {'file': filepath}
    patterns = complexity_patterns.get(language, {})
    
    for name, pattern in patterns.items():
        matches = re.findall(pattern, content)
        stats[name] = len(matches)
    
    # Calculate cyclomatic complexity (a simple approximation)
    complexity = (
        1  # Base complexity
        + stats.get('if', 0)
        + stats.get('else if', 0)
        + stats.get('elif', 0)
        + stats.get('for', 0)
        + stats.get('while', 0)
        + stats.get('case', 0)
        + stats.get('catch', 0)
        + stats.get('except', 0)
    )
    
    stats['complexity'] = complexity
    stats['high_complexity'] = complexity > max_complexity
    
    return stats

def main():
    metrics = []
    timestamp = datetime.utcnow().isoformat() + 'Z'
    
    # Analyse each language
    for language, patterns in file_patterns.items():
        files = find_files(repo_path, patterns)
        
        if not files:
            continue
            
        language_metrics = {
            'language': language,
            'file_count': len(files),
            'high_complexity_count': 0,
            'max_complexity': 0,
            'avg_complexity': 0,
            'complexity_details': []
        }
        
        total_complexity = 0
        
        for filepath in files:
            stats = calculate_complexity(filepath, language)
            if not stats:
                continue
                
            complexity = stats.get('complexity', 0)
            total_complexity += complexity
            
            relative_path = os.path.relpath(filepath, repo_path)
            language_metrics['complexity_details'].append({
                'file': relative_path,
                'complexity': complexity,
                'is_high': complexity > max_complexity
            })
            
            if complexity > language_metrics['max_complexity']:
                language_metrics['max_complexity'] = complexity
                
            if complexity > max_complexity:
                language_metrics['high_complexity_count'] += 1
        
        if language_metrics['file_count'] > 0:
            language_metrics['avg_complexity'] = total_complexity / language_metrics['file_count']
            
            # Add metrics for this language
            metrics.append({
                'name': f'{language}_file_count',
                'value': language_metrics['file_count'],
                'timestamp': timestamp
            })
            
            metrics.append({
                'name': f'{language}_avg_complexity',
                'value': language_metrics['avg_complexity'],
                'timestamp': timestamp
            })
            
            metrics.append({
                'name': f'{language}_max_complexity',
                'value': language_metrics['max_complexity'],
                'timestamp': timestamp
            })
            
            metrics.append({
                'name': f'{language}_high_complexity_count',
                'value': language_metrics['high_complexity_count'],
                'labels': {'threshold': str(max_complexity)},
                'timestamp': timestamp
            })
            
            # Add individual file metrics for high complexity files
            for detail in language_metrics['complexity_details']:
                if detail['is_high']:
                    metrics.append({
                        'name': 'high_complexity_file',
                        'value': detail['complexity'],
                        'labels': {
                            'file': detail['file'],
                            'language': language
                        },
                        'timestamp': timestamp
                    })
    
    # Output JSON
    print(json.dumps(metrics, indent=2))

if __name__ == '__main__':
    main()