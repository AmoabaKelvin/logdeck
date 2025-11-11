import { describe, it, expect } from 'vitest';
import { getContainerUrlIdentifier } from '../container-utils';
import type { ContainerInfo } from '../../types';

describe('getContainerUrlIdentifier', () => {
  it('should return container name without leading slash', () => {
    const container: ContainerInfo = {
      id: 'abc123def456',
      names: ['/my-container'],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    expect(result).toBe('my-container');
  });

  it('should return container name without slash when name has no slash', () => {
    const container: ContainerInfo = {
      id: 'abc123def456',
      names: ['my-container'],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    expect(result).toBe('my-container');
  });

  it('should return short ID when names array is empty', () => {
    const container: ContainerInfo = {
      id: 'abc123def456789',
      names: [],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    expect(result).toBe('abc123def456');
  });

  it('should handle container names with special characters', () => {
    const container: ContainerInfo = {
      id: 'abc123def456',
      names: ['/my-app_container-1'],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    expect(result).toBe('my-app_container-1');
  });

  it('should prioritize name over ID (main functionality)', () => {
    const container: ContainerInfo = {
      id: 'abc123def456789',
      names: ['/my-named-container'],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    // Should return name, NOT ID
    expect(result).not.toBe('abc123def456');
    expect(result).not.toBe('abc123def456789');
    expect(result).toBe('my-named-container');
  });

  it('should use first name when multiple names exist', () => {
    const container: ContainerInfo = {
      id: 'abc123def456',
      names: ['/primary-name', '/secondary-name'],
      image: 'nginx:latest',
      image_id: 'sha256:12345',
      command: 'nginx',
      created: 1234567890,
      state: 'running',
      status: 'Up 2 hours',
    };

    const result = getContainerUrlIdentifier(container);
    expect(result).toBe('primary-name');
  });
});
