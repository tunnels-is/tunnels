import React, { useEffect } from 'react';
import { useForm, useFieldArray } from 'react-hook-form';
import { 
  Form, 
  FormField, 
  FormItem, 
  FormLabel, 
  FormControl, 
  FormMessage 
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { PlusCircle, X } from "lucide-react";

const OrganizationForm = ({ 
  organizationData, 
  onSubmit, 
  onCancel, 
  formTitle,
  isCreate = false
}) => {
  const form = useForm({
    defaultValues: {
      Name: organizationData?.Name || '',
      Address: organizationData?.Address || '',
      Domains: organizationData?.Domains || ['myorg.local'],
      ManagerID: organizationData?.ManagerID || '',
      Information: organizationData?.Information || '',
      Email: organizationData?.Email || '',
      Phone: organizationData?.Phone || '',
      _id: organizationData?._id || undefined
    }
  });

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "Domains"
  });

  useEffect(() => {
    if (organizationData) {
      form.reset({
        Name: organizationData.Name || '',
        Address: organizationData.Address || '',
        Domains: organizationData.Domains || ['myorg.local'],
        ManagerID: organizationData.ManagerID || '',
        Information: organizationData.Information || '',
        Email: organizationData.Email || '',
        Phone: organizationData.Phone || '',
        _id: organizationData._id
      });
    }
  }, [organizationData, form.reset]);

  const handleFormSubmit = (data) => {
    onSubmit(data);
  };

  const addNewDomain = () => {
    append("new-domain.local");
  };

  return (
    <Card className={`org-form-card w-full max-w-4xl mx-auto ${isCreate ? 'create-form' : 'update-form'}`}>
      <CardHeader>
        <CardTitle>{formTitle || (isCreate ? 'Create Organization' : 'Edit Organization')}</CardTitle>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(handleFormSubmit)} className="space-y-6">
            <FormField
              control={form.control}
              name="Name"
              rules={{ required: "Organization name is required" }}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Organization Name</FormLabel>
                  <FormControl>
                    <Input placeholder="Enter organization name" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="Email"
              rules={{ 
                required: "Email is required",
                pattern: {
                  value: /^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$/i,
                  message: "Invalid email address"
                }
              }}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Email</FormLabel>
                  <FormControl>
                    <Input type="email" placeholder="organization@example.com" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="Phone"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Phone</FormLabel>
                  <FormControl>
                    <Input placeholder="Phone number" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="Address"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Address</FormLabel>
                  <FormControl>
                    <Textarea placeholder="Organization address" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="ManagerID"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Manager ID</FormLabel>
                  <FormControl>
                    <Input placeholder="Manager ID" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className="space-y-2">
              <FormLabel>Domains</FormLabel>
              <div className="space-y-3">
                {fields.map((field, index) => (
                  <div key={field.id} className="flex items-center gap-3">
                    <FormField
                      control={form.control}
                      name={`Domains.${index}`}
                      render={({ field }) => (
                        <FormItem className="flex-1 m-0">
                          <FormControl>
                            <Input {...field} />
                          </FormControl>
                        </FormItem>
                      )}
                    />
                    {index > 0 && (
                      <Button 
                        type="button" 
                        variant="ghost" 
                        size="icon"
                        onClick={() => remove(index)}
                        className="h-9 w-9"
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                ))}
                <Button 
                  type="button" 
                  variant="outline" 
                  size="sm"
                  onClick={addNewDomain}
                  className="flex items-center gap-1"
                >
                  <PlusCircle className="h-4 w-4" />
                  Add Domain
                </Button>
              </div>
            </div>

            <FormField
              control={form.control}
              name="Information"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Additional Info</FormLabel>
                  <FormControl>
                    <Textarea placeholder="Additional information" {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {!isCreate && (
              <FormField
                control={form.control}
                name="_id"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>ID</FormLabel>
                    <FormControl>
                      <Input disabled {...field} />
                    </FormControl>
                  </FormItem>
                )}
              />
            )}

            <CardFooter className="px-0 pt-2 pb-0 flex justify-end gap-3">
              {onCancel && (
                <Button type="button" variant="outline" onClick={onCancel}>
                  Cancel
                </Button>
              )}
              <Button type="submit">
                {isCreate ? 'Create Organization' : 'Update Organization'}
              </Button>
            </CardFooter>
          </form>
        </Form>
      </CardContent>
    </Card>
  );
};

export default OrganizationForm;