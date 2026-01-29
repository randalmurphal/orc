/**
 * Connect RPC client configuration
 *
 * Single transport instance with typed service clients for all orc services.
 */

import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

// Import services from _pb.ts files (GenService format for Connect v2)
import { TaskService } from '@/gen/orc/v1/task_pb';
import { EventService } from '@/gen/orc/v1/events_pb';
import { InitiativeService } from '@/gen/orc/v1/initiative_pb';
import { WorkflowService } from '@/gen/orc/v1/workflow_pb';
import { ConfigService } from '@/gen/orc/v1/config_pb';
import { DashboardService } from '@/gen/orc/v1/dashboard_pb';
import { ProjectService, BranchService } from '@/gen/orc/v1/project_pb';
import { TranscriptService } from '@/gen/orc/v1/transcript_pb';
import { AutomationService } from '@/gen/orc/v1/automation_pb';
import { HostingService } from '@/gen/orc/v1/hosting_pb';
import { DecisionService } from '@/gen/orc/v1/decision_pb';
import { NotificationService } from '@/gen/orc/v1/notification_pb';
import { MCPService } from '@/gen/orc/v1/mcp_pb';

/**
 * Connect transport configured for the orc API.
 * Requests go to same origin at /orc.v1.* paths.
 */
const transport = createConnectTransport({
	baseUrl: '',
});

// Service clients - typed wrappers around the transport
export const taskClient = createClient(TaskService, transport);
export const eventClient = createClient(EventService, transport);
export const initiativeClient = createClient(InitiativeService, transport);
export const workflowClient = createClient(WorkflowService, transport);
export const configClient = createClient(ConfigService, transport);
export const dashboardClient = createClient(DashboardService, transport);
export const projectClient = createClient(ProjectService, transport);
export const branchClient = createClient(BranchService, transport);
export const transcriptClient = createClient(TranscriptService, transport);
export const automationClient = createClient(AutomationService, transport);
export const hostingClient = createClient(HostingService, transport);
export const decisionClient = createClient(DecisionService, transport);
export const notificationClient = createClient(NotificationService, transport);
export const mcpClient = createClient(MCPService, transport);
