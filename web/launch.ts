const executablePath = process.env.MM_CHROMIUM;

export const launchOptions = executablePath ? { executablePath } : {};
