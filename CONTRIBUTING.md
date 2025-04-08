# Contributing to Mindnest

We welcome contributions to Mindnest!  Here's how you can help improve the project.

## How to Contribute

1.  **Fork the Repository:**  Start by forking the Mindnest repository to your own GitHub account.

2.  **Clone Locally:** Clone your forked repository to your local machine:

    ```bash
    git clone https://github.com/YOUR_USERNAME/mindnest.git
    cd mindnest
    ```

3.  **Create a Branch:** Create a new branch for your feature or bug fix:

    ```bash
    git checkout -b feature/your-feature-name
    # or
    git checkout -b fix/your-bug-fix
    ```

4.  **Make Changes:** Implement your changes, following the coding style and guidelines outlined below.

5.  **Test Your Changes:** Run the tests to ensure your changes haven't introduced any regressions:

    ```bash
    make test
    ```

    If you're adding a new feature, please include corresponding tests.

6.  **Format and Lint Your Code:** Before submitting, format your code and run the linters:

    ```bash
    make format
    make lint
    ```

7.  **Commit Your Changes:** Commit your changes with a clear and concise message:

    ```bash
    git commit -m "feat: Add amazing new feature"
    # or
    git commit -m "fix: Resolve issue with database connection"
    ```

8.  **Push to Your Fork:** Push your branch to your forked repository:

    ```bash
    git push origin feature/your-feature-name
    ```

9.  **Create a Pull Request:**  Submit a pull request from your branch to the `main` branch of the Mindnest repository.  Provide a clear description of your changes and their purpose.

## Coding Style and Guidelines

*   Mindnest follows the standard Go coding conventions.
*   Use `gofmt` and `golangci-lint` to format and lint your code (as enforced by `make format` and `make lint`).
*   Write clear and concise commit messages.
*   Document your code where appropriate.
*   Keep pull requests focused on a single feature or bug fix.

## Development Setup

The following tools are required for local development:

*   [Go](https://go.dev/dl/) (version 1.24 or later)
*   [SQLite](https://www.sqlite.org/download.html)
*   [Git](https://git-scm.com/downloads)
*   [Ollama](https://github.com/ollama/ollama)
*   (Optional) [Claude API Key](https://console.anthropic.com/)
*   (Optional) [Gemini API Key](https://console.google.com/gemini)
*   (Optional) [GitHub API Key](https://github.com/settings/tokens)

Refer to the [Installation](#installation) section of the [README.md](README.md) for detailed setup instructions.

Useful `make` commands:

*   `make build`: Builds the binary.
*   `make test`: Runs tests.
*   `make format`: Formats the code.
*   `make lint`: Runs linters.
*   `make deps`: Downloads dependencies.

## Reporting Bugs

If you find a bug, please create a new issue on GitHub with a clear description of the problem, including steps to reproduce it.

## Suggesting Enhancements

If you have an idea for a new feature or enhancement, please create a new issue on GitHub to discuss it.

## License

By contributing to Mindnest, you agree that your contributions will be licensed under the [MIT License](LICENSE).