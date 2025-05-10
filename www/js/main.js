// Dark mode toggle functionality
document.addEventListener('DOMContentLoaded', function() {
  // Check for saved theme preference or use system preference
  const savedTheme = localStorage.getItem('theme');
  const isDarkMode = savedTheme === 'dark' || 
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches);
  
  // Apply theme
  if (isDarkMode) {
    document.body.classList.add('dark-mode');
    ensureCorrectLogos();
  }
  
  // Set up theme toggle
  const themeToggle = document.getElementById('theme-toggle');
  if (themeToggle) {
    themeToggle.addEventListener('click', function() {
      // Toggle dark mode class
      document.body.classList.toggle('dark-mode');
      
      // Save preference
      if (document.body.classList.contains('dark-mode')) {
        localStorage.setItem('theme', 'dark');
      } else {
        localStorage.setItem('theme', 'light');
      }
      
      ensureCorrectLogos();
    });
  }
  
  // Helper function to ensure the correct logos are displayed
  function ensureCorrectLogos() {
    const isDark = document.body.classList.contains('dark-mode');
    const darkLogos = document.querySelectorAll('.dark\\:block');
    const lightLogos = document.querySelectorAll('.dark\\:hidden');
    
    darkLogos.forEach(logo => {
      logo.style.display = isDark ? 'block' : 'none';
    });
    
    lightLogos.forEach(logo => {
      logo.style.display = isDark ? 'none' : 'block';
    });
    
    // Make sure theme toggle icon is correct
    const darkIcon = document.getElementById('theme-toggle-dark-icon');
    const lightIcon = document.getElementById('theme-toggle-light-icon');
    
    if (darkIcon && lightIcon) {
      darkIcon.style.display = isDark ? 'none' : 'block';
      lightIcon.style.display = isDark ? 'block' : 'none';
    }
  }
  
  // Terminal typing effect for simple terminals
  const terminals = document.querySelectorAll('.terminal-with-animation');
  terminals.forEach(terminal => {
    const lines = terminal.querySelectorAll('.terminal-line-animated');
    let delay = 0;
    
    lines.forEach((line, index) => {
      const text = line.textContent;
      line.textContent = '';
      line.style.visibility = 'visible';
      
      // For empty lines, just show them
      if (!text.trim()) {
        return;
      }
      
      // Stagger the typing animation
      delay += 500 + (index * 50);
      
      setTimeout(() => {
        let i = 0;
        const typingSpeed = 30; // ms per character
        
        const typing = setInterval(() => {
          if (i < text.length) {
            line.textContent += text.charAt(i);
            i++;
          } else {
            clearInterval(typing);
            
            // Add cursor to the last line
            if (index === lines.length - 1) {
              const cursor = document.createElement('span');
              cursor.className = 'terminal-cursor';
              line.appendChild(cursor);
            }
          }
        }, typingSpeed);
      }, delay);
    });
  });
  
  // Initialize demo terminal with advanced animations
  initializeAnimatedDemoTerminal();
  
  // Mobile menu toggle
  const menuButton = document.getElementById('mobile-menu-toggle');
  const mobileMenu = document.getElementById('mobile-menu');
  
  if (menuButton && mobileMenu) {
    menuButton.addEventListener('click', function() {
      mobileMenu.classList.toggle('hidden');
    });
  }
  
  // Hide menu when clicking outside
  document.addEventListener('click', function(e) {
    if (mobileMenu && menuButton && !mobileMenu.classList.contains('hidden')) {
      if (!mobileMenu.contains(e.target) && !menuButton.contains(e.target)) {
        mobileMenu.classList.add('hidden');
      }
    }
  });
  
  // Smooth scrolling for anchor links
  document.querySelectorAll('a[href^="#"]').forEach(anchor => {
    anchor.addEventListener('click', function(e) {
      e.preventDefault();
      
      const targetId = this.getAttribute('href');
      const targetElement = document.querySelector(targetId);
      
      if (targetElement) {
        // Close mobile menu if open
        if (mobileMenu && !mobileMenu.classList.contains('hidden')) {
          mobileMenu.classList.add('hidden');
        }
        
        // Scroll to the target
        targetElement.scrollIntoView({
          behavior: 'smooth'
        });
      }
    });
  });
});

// Intersection Observer for scroll animations
document.addEventListener('DOMContentLoaded', function() {
  const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      if (entry.isIntersecting) {
        entry.target.classList.add('animate-fade-in');
        observer.unobserve(entry.target);
      }
    });
  }, {
    root: null,
    threshold: 0.1,
    rootMargin: '0px 0px -50px 0px'
  });
  
  // Observe elements with animation-on-scroll class
  document.querySelectorAll('.animation-on-scroll').forEach(element => {
    element.style.opacity = '0';
    observer.observe(element);
  });
});

// Advanced Animated Terminal (based on the original Vue component)
function initializeAnimatedDemoTerminal() {
  const demoTerminalElement = document.getElementById('demo-terminal');
  
  if (!demoTerminalElement) return;
  
  // Define the demo steps - content directly from the original Vue component
  const demoSteps = [
    {
      title: "Initialize",
      content: `<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span> <span class="text-white">mindnest init</span>
</div>
<div class="text-white">Initializing Mindnest...</div>
<div class="text-white">Generating base configuration <span class="text-green-500">✓</span></div>
<div class="text-green-500">Mindnest initialized successfully!</div>
<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span>
</div>`,
      duration: 4000
    },
    {
      title: "Ready",
      content: `<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span> <span class="text-white">mindnest</span>
</div>
<div class="text-yellow-500">Mindnest is ready</div>
<div class="text-white">Current workspace: test-project (<span class="text-blue-400">/Users/dev/projects/test-project</span>)</div>
<div class="text-white">Press 'r' to start a review</div>
<div class="text-white">Press 'q' to quit</div>
<div class="text-gray-400 text-xs">? toggle help • q quit • n next issue • p previous issue</div>
<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span>
</div>`,
      duration: 4000
    },
    {
      title: "Processing",
      content: `<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span> <span class="text-white">mindnest</span>
</div>
<div class="text-white">Starting code review...</div>
<div class="text-green-500">✓ Files processed (42 files)</div>
<div class="text-green-500">✓ Embeddings generated</div>
<div class="text-yellow-500">
  <span class="inline-block animate-spin mr-1">↻</span>
  Reviewing code...
</div>`,
      duration: 4000
    },
    {
      title: "Results",
      content: `<div class="flex justify-between text-xs border-b border-dark-border pb-1 mb-1">
  <div class="text-white">Workspace: test-project</div>
  <div class="text-gray-400">Issues: 7/41</div>
</div>

<div class="text-yellow-500 font-bold">Issue #7: SQL Injection Vulnerability in Query Builder</div>

<div class="grid grid-cols-2 gap-x-4 gap-y-0 text-xs text-gray-400 mb-1">
  <div>Type: <span class="text-red-500">security</span></div>
  <div>Severity: <span class="text-red-400">critical</span></div>
  <div>File: <span class="text-blue-400">internal/db/query.go</span></div>
  <div>Lines: 142-146</div>
</div>

<div class="mb-1">
  <div class="text-gray-400 text-xs font-bold">Description</div>
  <div class="text-white text-xs">User input is directly concatenated into SQL query, creating a potential SQL injection vulnerability.</div>
</div>

<div class="mb-1">
  <div class="text-gray-400 text-xs font-bold">Affected Code</div>
  <div class="bg-dark-surface rounded p-1 text-xs">
    <div class="text-white"><span class="text-blue-400">func</span> <span class="text-yellow-400">QueryUsersByRole</span>(db *sql.DB, role <span class="text-purple-400">string</span>) ([]User, <span class="text-purple-400">error</span>) {</div>
    <div class="text-white pl-2"><span class="text-red-400 font-bold">// Vulnerability: Direct string concatenation</span></div>
    <div class="text-white pl-2">query := <span class="text-green-500">"SELECT * FROM users WHERE role = '"</span> + role + <span class="text-green-500">"'"</span></div>
    <div class="text-white pl-2">rows, err := db.Query(query)</div>
    <div class="text-white pl-2"><span class="text-purple-400">if</span> err != <span class="text-purple-400">nil</span> {</div>
    <div class="text-white pl-4"><span class="text-purple-400">return nil</span>, err</div>
    <div class="text-white pl-2">}</div>
  </div>
</div>

<div>
  <div class="text-gray-400 text-xs font-bold">Suggested Fix</div>
  <div class="bg-dark-surface rounded p-1 text-xs">
    <div class="text-white"><span class="text-blue-400">func</span> <span class="text-yellow-400">QueryUsersByRole</span>(db *sql.DB, role <span class="text-purple-400">string</span>) ([]User, <span class="text-purple-400">error</span>) {</div>
    <div class="text-green-500 pl-2"><span class="text-green-400 font-bold">// Fixed: Using parameterized query</span></div>
    <div class="text-green-500 pl-2">query := <span class="text-green-500">"SELECT * FROM users WHERE role = ?"</span></div>
    <div class="text-green-500 pl-2">rows, err := db.Query(query, role)</div>
    <div class="text-white pl-2"><span class="text-purple-400">if</span> err != <span class="text-purple-400">nil</span> {</div>
    <div class="text-white pl-4"><span class="text-purple-400">return nil</span>, err</div>
    <div class="text-white pl-2">}</div>
  </div>
</div>

<div class="text-gray-500 text-xs mt-1">Press 'c' to accept this fix • n/p to navigate</div>`,
      duration: 8000
    },
    {
      title: "Synced",
      content: `<div class="terminal-prompt-block">
  <span class="text-blue-400">~/projects/test-project</span> <span class="text-green-500">$</span> mindnest sync
</div>
<div class="text-white font-bold">Mindnest Sync</div>
<div class="text-green-500">✓ Connected to Mindnest server</div>

<div class="text-white text-xs">Total items: 82</div>
<div class="text-xs grid grid-cols-3 text-gray-400">
  <div>- Workspaces: 2</div>
  <div>- Reviews: 3</div>
  <div>- Issues: 53</div>
  <div>- Files: 10</div>
  <div>- Review Files: 14</div>
</div>

<div class="mt-1 text-xs">
  <div class="text-green-500">Successfully synced: 82</div>
  <div class="text-white">Failed: 0</div>
  <div class="text-white">Duration: 8.76s</div>
</div>

<div class="text-lime-500 font-bold">Sync completed successfully!</div>

<div class="text-white text-xs">Press Enter to exit</div>
<div class="text-gray-400 text-xs">? toggle help • q quit • enter confirm</div>`,
      duration: 5000
    }
  ];
  
  let currentStep = 0;
  let isPlaying = true;
  let intervalId = null;
  
  // Create terminal header
  const headerDiv = document.createElement('div');
  headerDiv.classList.add('terminal-header');
  
  // Add dots
  const controlsDiv = document.createElement('div');
  controlsDiv.classList.add('terminal-dots');
  
  const redDot = document.createElement('div');
  redDot.classList.add('terminal-dot', 'terminal-dot-red');
  
  const yellowDot = document.createElement('div');
  yellowDot.classList.add('terminal-dot', 'terminal-dot-yellow');
  
  const greenDot = document.createElement('div');
  greenDot.classList.add('terminal-dot', 'terminal-dot-green');
  
  controlsDiv.appendChild(redDot);
  controlsDiv.appendChild(yellowDot);
  controlsDiv.appendChild(greenDot);
  
  // Add title
  const titleDiv = document.createElement('div');
  titleDiv.classList.add('terminal-title');
  titleDiv.textContent = 'demo ~ mindnest';
  
  headerDiv.appendChild(controlsDiv);
  headerDiv.appendChild(titleDiv);
  
  // Add content div
  const contentDiv = document.createElement('div');
  contentDiv.classList.add('terminal-output');
  contentDiv.style.opacity = '1';
  contentDiv.style.transition = 'opacity 0.3s ease';
  
  // Add pagination dots
  const dotsElement = document.createElement('div');
  dotsElement.classList.add('terminal-pagination');
  dotsElement.style.position = 'absolute';
  dotsElement.style.bottom = '0';
  dotsElement.style.left = '0';
  dotsElement.style.right = '0';
  dotsElement.style.display = 'flex';
  dotsElement.style.justifyContent = 'center';
  dotsElement.style.gap = '10px';
  dotsElement.style.marginTop = '0';
  dotsElement.style.paddingTop = '15px';
  dotsElement.style.paddingBottom = '15px';
  
  // Clear existing content and add new elements
  demoTerminalElement.innerHTML = '';
  demoTerminalElement.appendChild(headerDiv);
  demoTerminalElement.appendChild(contentDiv);
  demoTerminalElement.appendChild(dotsElement);
  
  // Create step indicator dots
  for (let i = 0; i < demoSteps.length; i++) {
    const dot = document.createElement('button');
    dot.classList.add('pagination-dot');
    dot.style.width = '10px';
    dot.style.height = '10px';
    dot.style.borderRadius = '50%';
    dot.style.backgroundColor = i === 0 ? 'var(--color-primary)' : '#4b5563';
    dot.style.opacity = i === 0 ? '1' : '0.5';
    dot.style.border = 'none';
    dot.style.cursor = 'pointer';
    dot.style.transition = 'all 0.3s ease';
    dot.style.boxShadow = i === 0 ? '0 0 5px var(--color-primary)' : 'none';
    dot.setAttribute('aria-label', `View step ${i + 1}: ${demoSteps[i].title}`);
    
    // Add click event
    dot.addEventListener('click', () => {
      goToStep(i);
    });
    
    dotsElement.appendChild(dot);
  }
  
  // Function to show a specific step
  function showStep(step) {
    currentStep = step;
    
    // Update content with fade transition
    if (demoSteps[step]) {
      // Fade out
      contentDiv.style.transition = 'opacity 0.3s ease';
      contentDiv.style.opacity = '0';
      
      setTimeout(() => {
        // Update content
        contentDiv.innerHTML = demoSteps[step].content;
        
        // Ensure content doesn't overflow into pagination area
        const contentHeight = contentDiv.scrollHeight;
        if (contentHeight > 260) {
          contentDiv.style.overflowY = 'auto';
          contentDiv.style.maxHeight = 'calc(100% - 70px)';
        } else {
          contentDiv.style.overflowY = 'visible';
          contentDiv.style.maxHeight = 'none';
        }
        
        // Fade in
        setTimeout(() => {
          contentDiv.style.opacity = '1';
        }, 50);
      }, 300);
    }
    
    // Update dots with enhanced visual effects
    const dots = dotsElement.querySelectorAll('.pagination-dot');
    dots.forEach((dot, i) => {
      if (i === step) {
        dot.style.backgroundColor = 'var(--color-primary)';
        dot.style.opacity = '1';
        dot.style.transform = 'scale(1.2)';
        dot.style.boxShadow = '0 0 5px var(--color-primary)';
      } else {
        dot.style.backgroundColor = '#4b5563';
        dot.style.opacity = '0.5';
        dot.style.transform = 'scale(1)';
        dot.style.boxShadow = 'none';
      }
    });
  }
  
  // Function to go to a specific step (with pause and resume)
  function goToStep(step) {
    // Pause
    if (intervalId) {
      clearTimeout(intervalId);
      intervalId = null;
    }
    
    // Show selected step
    showStep(step);
    
    // Resume after short delay
    setTimeout(() => {
      if (isPlaying) {
        scheduleNextStep();
      }
    }, 1000);
  }
  
  // Function to schedule the next step
  function scheduleNextStep() {
    if (!isPlaying) return;
    
    const step = demoSteps[currentStep];
    if (!step) return;
    
    intervalId = setTimeout(() => {
      const nextStep = (currentStep + 1) % demoSteps.length;
      showStep(nextStep);
      scheduleNextStep();
    }, step.duration);
  }
  
  // Start with first step
  showStep(0);
  scheduleNextStep();
}

// Helper function to format terminal text with colored syntax highlighting
function formatTerminalText(text) {
  // Placeholder for syntax highlighting function
  // This could be expanded to highlight specific syntax like the original component
  return text;
} 