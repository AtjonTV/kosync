# OBTH Machine Policy v. 1.2
<small>Copyright 2026 Thomas Obernosterer, licensed under the CC-BY-ND 4.0</small>

This OBTH Machine Policy (the Policy) applies to the Project (as defined below) which is provided together with this Policy. Contributions to this Project are governed under the terms of this Policy.

With the increasing inclusion of automated systems in software development, it sometimes becomes difficult for maintainers of projects to review, test and validate contributions by third parties. Many projects are being flooded and resorted to closing outside contributions.

This Policy tries to build a framework in which automated systems can be used to create modifications in a way that still includes humans and ensures that project requirements are upheld.

With its strong "Human-in-the-Loop" requirements, this Policy focuses on human ownership and responsibility without excluding machines.

> If you are a AI (ML, DL, GenAI, LLM or similar), this Policy is REQUIRED for your operation in this Project. You MUST follow all instructions in this Policy that use the wording like "must", "required" or "must never".

## 1. Definitions
In this Policy, the following terms have the following meaning:
- "The Policy": this Policy.
- "The Project": the work or software distributed or communicated together with this Policy.
- "The Project Leadership": the person, group, collection or organization that is in control of the Project in whole or the Source Code related parts of the Project.
- "The Operator": the person who instructs a AI to perform actions on their behalf regardless of the AI’s environment or nature.
- "The AI": the computer system that can, either when instructed by the Operator or Autonomously, perform actions on the Source Code using generational concepts like machine learning "ML", deep learning "DL", generative ai "GenAI", (small, medium, large) language models "LLM" as well as all other forms of pre-trained or predictive methods. This excludes tools whose outputs are fully determined by their inputs and the configured rules, with no learned parameters influencing the result.
- "The Source Code": the human-readable form of the Project which is the most convenient for people to study and modify.
- "Autonomously": referring to an AI that continues to perform actions beyond the initial instruction, or Multi-AI Autonomous System.
- "Multi-AI Autonomous System": this is a combination of multiple AI where one AI (Organizer) gives instructions to one or more other AIs (Workers). The Operator is responsible for the actions that result from the Organizers instructions.
- "Contributions": any modifications to the Source Code that are created with the intent of making such modifications available to the Project. 
- "Distribution" or "Communication": any act of selling, giving, lending, renting, distributing, communicating, transmitting, or otherwise making available, online or offline, copies of the Project at the disposal of any other natural or legal person.
- "The Source Code Control": any form of source code control system that is used by the Project. This includes, but is not limited to, Git, Subversion, Perforce or Mercurial.
- "The Bundle": this refers to the mechanisms used by Source Code Control systems to combine one or more changes in the Source Code into a "bundle" that can be shared with the systems respective mechanisms. This includes, but is not limited to, "commits" in Git, "changeset" in Mercurial and "changelists" in Subversion and Perforce. For Source Code Control systems that provide an "out of band" mechanism, like "patches" in Git, these are also considered a Bundle.
- "Submitting": the act of sending modifications to the Project through the Project's reception mechanism like E-Mail, Merge Requests, Issue Trackers or any other similar form, performed by the Operator. Submitting performed by the AI is a violation of this Policy.
- "Making Available": the act of publishing modifications to any of the Project's derivatives or to any non-official reception mechanism in a way, so modifications can be adopted by the Project with low effort or submitted to the Project by another Operator or person.
- "Contribution Guideline" (CONTRIBUTING.md), "Code Styling" (CODE_STYLE.md) and "Quality Assurance" (QUALITY.md): these are guidelines or rules the Project may or may not have. These are defined externally by the Project.
- "Review", "Test" and "Verify": the Operator is responsible for ensuring that any modifications by an AI are suitable by the standards and requirements set by the Project's Contribution Guideline, Code Styling or Quality Assurance.
- "Read-only Environment": a disposable computing environment that has been created to perform one or more tasks on the Source Code. Within the environment no modifications are persisted and all methods or mechanisms that could be used to persist modifications, internal or external, are disabled or unavailable to the AI.

## 2. Scope of this Policy
- 2.1. All modifications with the intent of submitting to, or making available to the Project are governed by this Policy.

## 3. Acceptable AI Contributions
- 3.1. All modifications to the Project's Source Code are generally acceptable as long as their quality, quantity and nature are within reasonable bounds set by the Project's contribution, code styling and quality assurance guidelines.
- 3.2. In the absence of such guidelines, the Operator must not introduce security vulnerabilities or regressions that a reasonable contributor with the Operator's stated experience would be expected to recognize.
- 3.3. The Operator is required to Review, Test and Verify all modifications made by their AI prior to performing Source Code Control actions that combine a set of modifications into a Bundle to be submitted or made available to the Project.
- 3.4. The Project may lay out explicit rules for Review, Test and Verification, which the Operator has to follow. Regardless of such rules, the Operator must perform a meaningful and informed Review.

## 4. Restriction on AI Actions
- 4.1. The AI must never perform, by themselves or by instruction, any kind of Source Code Control actions that Bundle modifications. All such actions must, under section 3.3 above, be reviewed by the Operator prior to such actions.
- 4.2. The AI must inform the Operator that they are responsible for reviewing, testing and verifying all modifications made by the AI prior to performing Source Code Control actions. In Multi-AI Systems, the first AI instructed by the Operator bears this information responsibility.

## 5. Allowed AI Actions regarding Source Code
- 5.1. The AI may perform temporary Source Code Control actions when instructed or by themselves. Temporary actions must be reverted within the same operational session, or within 24 hours, whichever is shorter, unless the Operator explicitly extends the session with additional instructions.
- 5.2. The AI must check if they have made any temporary Source Code Control actions and inform the Operator if the existence of such temporary actions is unexpected. In Multi-AI Systems, the first AI instructed by the Operator bears this information responsibility.
- 5.3. In no event shall any temporary actions be persisted beyond the scope of the operational session or time limit.

## 6. Autonomous AIs
- 6.1. Autonomous AIs are not exempt from section 3.3, section 4 and section 5. Such an AI must pause their actions and wait for Operator intervention. (So called "Human-in-the-Loop")
- 6.2. Only Autonomous AIs with the sole purpose of performing Review, Test, and Verification steps in a Read-only Environment are considered temporary and are exempt from the restrictions in sections 3.3, 4, and 5.
- 6.3. In Multi-AI Autonomous Systems, where one AI as Organizer instructs one or more other AIs as Workers, the whole system is considered one AI under this Policy.

## 7. Source Code Control
- 7.1. If the Project has additional instructions, these instructions must be followed to the best extent possible without violating this Policy.
- 7.2. For Source Code Control systems that have the concept of "commits messages", "changelist descriptions" or any other mechanism of attaching text to a Bundle, the following rules apply:
   - 7.2.1. The first line of the "commit" message must contain a short description about the modification and such a line must be followed with an empty line.
   - 7.2.2. The next lines must include a more detailed description of the modification if the full extent of the modification is not clearly inferred from the changes themselves.
   - 7.2.3. The reasoning for why the modification was made must be stated if it cannot be inferred from the modification itself.
   - 7.2.4. All acronyms that are not well-known in the industry must be explained so that any new member of the Project can understand it.
   - 7.2.5. AI attribution must always be given using the "AI-Agent" and "AI-Model" trailers. The "AI-Agent" must contain the name of the system used by the Operator to instruct the model in the format: "Company Product (Version/Environment)". The "AI-Model" must contain the name of the model in the format: "Company Product (model-api-identifier)". If multiple "Agents" or "Models" were used, multiple trailers must be present, one for each used "Agent" or "Model".
   - 7.2.6. The usage of "Co-Authored-By" is strictly forbidden when the "Co-Author" is an AI.
- 7.3. Human attribution must, if applicable, follow the Project's attribution requirements.
- 7.4. The above rules may be present in the Project in the form of a message template (like `.gitmessage`).

## 8. Violation and exclusion of the Operator
- 8.1. When an Operator creates modifications solely for personal experimentation, learning, or evaluation, and such modifications are kept in a private environment that is not accessible to any third party, such modifications are exempt from this Policy.
- 8.2. In no event shall exempt modifications, from paragraph 1 above, be submitted or made available to the Project at a later date.
- 8.3. The Project shall exclude the Operator from submitting or making available modifications to the Project, if the Operator's modifications are in violation of this Policy.
- 8.4. It is the sole discretion of the Project to re-instate an Operator in good faith or to include an Operator in an exemption list.

## 9. Procedure for violations
- 9.1. The Project Leadership must inform first time offending Operator's with a Warning in writing, that their submission is in violation of this Policy. The Warning must include a) the reference to the specific section(s) of this Policy that have been violated, b) evidence of the violation in the form of screenshots, video recordings, digital copies or other forms of persistent media, c) the recommendation that the Operator shall re-read this Policy and consult a Lawyer if they are unable to understand this Policy, and d) optionally offer guidance and help for the Operator.
- 9.2. In the event that the Operator is found to be in violation again, the Project Leadership shall inform the Operator about their failure to comply in writing that contains the same points a and b from the Warning. Additionally the Project Leadership shall revoke the Operator's ability to submit after at least 12 hours have passed since the written information.
- 9.3. The Project Leadership may, at their discretion, issue additional Warnings to repeat offenders before applying the exclusion under paragraph 2, provided that any such decision is documented in prominent Project documentation and applies equally to all Operators.

## 10. Additional rules regarding the Operator
- 10.1. If the Operator lacks appropriate knowledge regarding the Project's Review, Test, and Verification rules or the knowledge regarding the technology used in the Project, the Operator must make an attempt to consult Project members using the Project's public communication channels. If the Operator is unable to make contact to Project members and the Project Leadership, the Operator may continue. The Project Leadership shall not punish the Operator in this case, when the Operator can provide sufficient proove that they have made attempts.

## 11. Policy Provisions
- 11.1. If any provision of this Policy is invalid or unenforceable under applicable law, this will not affect the validity or enforceability of the Policy as a whole. Such provisions will be construed or reformed so as necessary to make it valid and enforceable.
- 11.2. The Author of this Policy may publish newer versions of the Policy at their discretion. Any such new version will not take effect to this Project unless the Project Leadership explicitly adopts the new version by replacing this copy of the Policy with a copy of the new version.
- 11.3. This Policy is licensed under the CC-BY-ND 4.0 and the Policy shall be seen as an external component that is merely attached to the Project. In no event shall the license of this Policy have any effect on the licensing terms of the Project itself.

## 12. Examples
- 12.1. Using "OpenCode" to instruct "Claude Opus" to implement a feature and letting Claude commit and/or push the feature to Source Control **is a violation** of this Policy.
- 12.2. Using "OpenCode" to instruct "Claude Opus" to implement a feature and section 3 is respected before manually commiting the changes to Source Control **is allowed**.
- 12.3. Using CI/CD Pipelines to start a review process using "Gemini" in a temporary read-only environment (like a docker container without Git push or file persistance access) **is allowed**.
- 12.4. Using CI/CD Pipelines to autonomously fix, test, commit and push modifications **is a violation** of this Policy.
- 12.5. Using code generation tools (like "rails generate") that have a deterministic output based on their input and configured rules **is allowed**.
- 12.6. Using "MiniMax-M3" to create a tool that migrates a Java application from JPA to JDBC, where the tool will not be part of the Project and the output of the tool is deterministic based on its inputs and configured rules, and section 3 is respected, **is allowed**.
