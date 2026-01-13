/**
 * Mock for $app/environment
 */

// In tests, we simulate browser environment
export const browser = true;

// Not building for production in tests
export const building = false;

// Not in dev mode during tests
export const dev = false;

// Version can be any string for tests
export const version = 'test';
